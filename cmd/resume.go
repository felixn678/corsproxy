package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"charm.land/huh/v2"
	"github.com/felix-nguyen/corsproxy/internal/config"
	"github.com/felix-nguyen/corsproxy/internal/session"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume [name|uuid]",
	Short: "Restart a previously saved server",
	Long: `Restart a server saved from a previous run, with the same target, port,
and origin. With no argument, pick one from a list of saved servers.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runResume,
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

func runResume(cmd *cobra.Command, args []string) error {
	sessions := loadSessionsLenient()
	if len(sessions) == 0 {
		return errors.New("no saved servers yet — start one first: corsproxy --target http://localhost:8080")
	}

	var sess session.Session
	if len(args) == 1 {
		var err error
		sess, err = findSession(sessions, args[0])
		if err != nil {
			return err
		}
	} else {
		var err error
		sess, err = pickSession(sessions)
		if err != nil {
			return err
		}
	}

	// Re-validate through config.New like a fresh start: a stale or
	// hand-edited stored config must fail loudly, not mysteriously.
	cfg, err := config.New(sess.Target, sess.Port, sess.Origin)
	if err != nil {
		return fmt.Errorf("saved server %q has invalid config: %w", resumeKey(sess), err)
	}
	// Same start path as a normal run: bumps lastUsedAt via persistSession
	// and prints a fresh resume hint on exit. Port-busy reuses the existing
	// suggest-next-port error in runServer.
	return runServer(cfg, sess.Name)
}

// loadSessionsLenient loads the store with the same warn-and-continue
// handling as start-up: a corrupt store is an empty store, never a crash.
func loadSessionsLenient() []session.Session {
	path, err := session.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ Cannot locate session store: %v\n", err)
		return nil
	}
	sessions, err := session.NewStore(path).Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ %v — treating the store as empty\n", err)
		return nil
	}
	return sessions
}

// findSession resolves a user-supplied key with friendly errors for the two
// failure modes: nothing matched, or a name matched several servers.
func findSession(sessions []session.Session, key string) (session.Session, error) {
	sess, err := session.Find(sessions, key)

	var ambig *session.AmbiguousError
	switch {
	case errors.Is(err, session.ErrNotFound):
		return session.Session{}, fmt.Errorf(
			"no saved server matches %q — run \"corsproxy resume\" to pick from the list", key)
	case errors.As(err, &ambig):
		for _, m := range ambig.Matches {
			fmt.Fprintf(os.Stderr, "  %s  %s  → %s  :%d\n", m.Name, m.ID, m.Target, m.Port)
		}
		return session.Session{}, fmt.Errorf("name %q is ambiguous — resume by uuid instead", key)
	case err != nil:
		return session.Session{}, err
	}
	return sess, nil
}

// pickSession shows a select list of saved servers, most recently used first.
func pickSession(sessions []session.Session) (session.Session, error) {
	slices.SortFunc(sessions, func(a, b session.Session) int {
		return b.LastUsedAt.Compare(a.LastUsedAt) // descending: newest first
	})

	options := make([]huh.Option[session.Session], 0, len(sessions))
	for _, s := range sessions {
		label := s.Name
		if label == "" {
			// 8 uuid chars are enough to tell entries apart visually; the
			// full uuid still works as a resume argument. Guard the slice:
			// a hand-edited store may hold IDs shorter than 8 chars.
			label = s.ID
			if len(label) > 8 {
				label = label[:8]
			}
		}
		options = append(options,
			huh.NewOption(fmt.Sprintf("%s → %s  :%d", label, s.Target, s.Port), s))
	}

	var picked session.Session
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[session.Session]().
			Title("Resume which server?").
			Options(options...).
			Value(&picked),
	))
	if err := form.Run(); err != nil {
		return session.Session{}, fmt.Errorf("%w\n(no TTY? use: corsproxy resume <name|uuid>)", err)
	}
	return picked, nil
}
