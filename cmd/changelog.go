package cmd

import (
	stdctx "context"
	"fmt"
	"time"

	"github.com/caarlos0/log"
	"github.com/ostiwe/goreleaser/v2/internal/middleware/errhandler"
	"github.com/ostiwe/goreleaser/v2/internal/middleware/logging"
	"github.com/ostiwe/goreleaser/v2/internal/middleware/skip"
	"github.com/ostiwe/goreleaser/v2/internal/pipe/changelog"
	"github.com/ostiwe/goreleaser/v2/internal/skips"
	"github.com/ostiwe/goreleaser/v2/pkg/context"
	"github.com/spf13/cobra"
)

type changelogCmd struct {
	cmd  *cobra.Command
	opts releaseOpts
}

func newChangelogCmd() *changelogCmd {
	root := &changelogCmd{}
	//nolint:dupl
	cmd := &cobra.Command{
		Use:               "changelog",
		Aliases:           []string{"ch"},
		Short:             "Get changelog",
		SilenceUsage:      true,
		SilenceErrors:     true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getChangelog(cmd.Context(), root.opts)
		},
	}

	cmd.Flags().StringVarP(&root.opts.config, "config", "f", "", "Load configuration from file")
	_ = cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.Flags().StringVar(&root.opts.releaseNotesFile, "release-notes", "", "Load custom release notes from a markdown file (will skip GoReleaser changelog generation)")
	_ = cmd.MarkFlagFilename("release-notes", "md", "mkd", "markdown")
	cmd.Flags().StringVar(&root.opts.releaseHeaderFile, "release-header", "", "Load custom release notes header from a markdown file")
	_ = cmd.MarkFlagFilename("release-header", "md", "mkd", "markdown")
	cmd.Flags().StringVar(&root.opts.releaseFooterFile, "release-footer", "", "Load custom release notes footer from a markdown file")
	_ = cmd.MarkFlagFilename("release-footer", "md", "mkd", "markdown")
	cmd.Flags().StringVar(&root.opts.releaseNotesTmpl, "release-notes-tmpl", "", "Load custom release notes from a templated markdown file (overrides --release-notes)")
	_ = cmd.MarkFlagFilename("release-notes-tmpl", "md", "mkd", "markdown")
	cmd.Flags().StringVar(&root.opts.releaseHeaderTmpl, "release-header-tmpl", "", "Load custom release notes header from a templated markdown file (overrides --release-header)")
	_ = cmd.MarkFlagFilename("release-header-tmpl", "md", "mkd", "markdown")
	cmd.Flags().StringVar(&root.opts.releaseFooterTmpl, "release-footer-tmpl", "", "Load custom release notes footer from a templated markdown file (overrides --release-footer)")
	_ = cmd.MarkFlagFilename("release-footer-tmpl", "md", "mkd", "markdown")
	cmd.Flags().BoolVar(&root.opts.autoSnapshot, "auto-snapshot", false, "Automatically sets --snapshot if the repository is dirty")
	cmd.Flags().BoolVar(&root.opts.snapshot, "snapshot", false, "Generate an unversioned snapshot release, skipping all validations and without publishing any artifacts (implies --skip=announce,publish,validate)")
	cmd.Flags().BoolVar(&root.opts.draft, "draft", false, "Whether to set the release to draft. Overrides release.draft in the configuration file")
	cmd.Flags().BoolVar(&root.opts.failFast, "fail-fast", false, "Whether to abort the release publishing on the first error")
	cmd.Flags().BoolVar(&root.opts.clean, "clean", false, "Removes the 'dist' directory")
	cmd.Flags().IntVarP(&root.opts.parallelism, "parallelism", "p", 0, "Amount tasks to run concurrently (default: number of CPUs)")
	_ = cmd.RegisterFlagCompletionFunc("parallelism", cobra.NoFileCompletions)
	cmd.Flags().DurationVar(&root.opts.timeout, "timeout", time.Hour, "Timeout to the entire release process")
	_ = cmd.RegisterFlagCompletionFunc("timeout", cobra.NoFileCompletions)
	cmd.Flags().BoolVar(&root.opts.deprecated, "deprecated", false, "Force print the deprecation message - tests only")
	_ = cmd.Flags().MarkHidden("deprecated")
	cmd.Flags().StringSliceVar(
		&root.opts.skips,
		"skip",
		nil,
		fmt.Sprintf("Skip the given options (valid options are %s)", skips.Release.String()),
	)
	_ = cmd.RegisterFlagCompletionFunc("skip", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return skips.Release.Complete(toComplete), cobra.ShellCompDirectiveDefault
	})
	root.cmd = cmd

	return root
}

func getChangelog(parent stdctx.Context, options releaseOpts) error {
	start := time.Now()
	cfg, err := loadConfig(!options.snapshot, options.config)
	if err != nil {
		return decorateWithCtxErr(parent, err, "release", after(start))
	}

	ctx, cancel := context.WrapWithTimeout(parent, cfg, options.timeout)
	defer cancel()

	if err := setupReleaseContext(ctx, options); err != nil {
		return decorateWithCtxErr(ctx, err, "release", after(start))
	}

	pipe := changelog.Pipe{}

	if err := skip.Maybe(
		pipe,
		logging.Log(
			pipe.String(),
			errhandler.Handle(pipe.Run),
		),
	)(ctx); err != nil {
		return decorateWithCtxErr(ctx, err, "changelog", after(start))
	}

	deprecateWarn(ctx)
	log.Infof(boldStyle.Render(fmt.Sprintf("release succeeded after %s", after(start))))
	return nil
}
