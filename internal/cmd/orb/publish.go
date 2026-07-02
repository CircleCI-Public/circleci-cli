// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package orb

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/cmdutil"
	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <command>",
		Short: "Publish orb versions",
		Long: heredoc.Doc(`
			Publish orb versions to the CircleCI orb registry.

			To publish a specific version:
			  circleci orb publish path ns/orb@version

			To promote a dev version to a stable semver:
			  circleci orb publish promote ns/orb@dev:label --bump major|minor|patch

			To increment the latest stable version and publish:
			  circleci orb publish increment path ns/orb --bump major|minor|patch
		`),
		Example: heredoc.Doc(`
			# Publish a specific version
			$ circleci orb publish orb.yml myorg/my-orb@1.0.0

			# Publish a dev version
			$ circleci orb publish orb.yml myorg/my-orb@dev:my-branch

			# Promote a dev version to a patch release
			$ circleci orb publish promote myorg/my-orb@dev:my-branch --bump patch

			# Increment major version and publish
			$ circleci orb publish increment orb.yml myorg/my-orb --bump major
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If first arg looks like "promote" or "increment", delegate to subcommand via GroupRunE.
			// Otherwise, treat as: publish <path> <ns>/<orb>@<version>
			if len(args) > 0 && (args[0] == "promote" || args[0] == "increment") {
				return cmdutil.GroupRunE(cmd, args)
			}
			if len(args) < 2 {
				return cmdutil.GroupRunE(cmd, args)
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOrbPublish(ctx, client, args[0], args[1])
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}

	cmd.AddCommand(newPublishPromoteCmd())
	cmd.AddCommand(newPublishIncrementCmd())

	return cmd
}

func newPublishPromoteCmd() *cobra.Command {
	var bump string

	cmd := &cobra.Command{
		Use:   "promote <ns>/<orb>@dev:<label> --bump major|minor|patch",
		Short: "Promote a dev orb version to a stable semver",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				- ns/orb@dev:label: the dev version to promote, e.g. "namespace/orb-name@dev:my-branch"
			`),
		},
		Long: heredoc.Doc(`
			Promote a dev orb version to a stable semver version.

			The dev version ref must be in the form ns/orb@dev:label.
			The --bump flag determines how the version is incremented
			from the latest stable release.
		`),
		Example: heredoc.Doc(`
			# Promote dev:my-branch to a patch release
			$ circleci orb publish promote myorg/my-orb@dev:my-branch --bump patch

			# Promote to a minor release
			$ circleci orb publish promote myorg/my-orb@dev:my-branch --bump minor

			# Promote to a major release
			$ circleci orb publish promote myorg/my-orb@dev:my-branch --bump major
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "ns/orb@dev:label"); err != nil {
				return err
			}
			if !isValidSegment(bump) {
				return clierrors.New("args.invalid_segment", "Invalid semver segment",
					"--bump must be one of: major, minor, patch").
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOrbPromote(ctx, client, args[0], bump)
		},
	}

	cmd.Flags().StringVar(&bump, "bump", "", "which version segment to increment: major, minor, or patch")

	return cmd
}

func newPublishIncrementCmd() *cobra.Command {
	var bump string

	cmd := &cobra.Command{
		Use:   "increment <path> <ns>/<orb> --bump major|minor|patch",
		Short: "Increment and publish a new orb version",
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
				- path: path to the orb YAML to publish. Pass '-' to read from stdin.
				- ns/orb: the orb to publish, as "namespace/orb-name"
			`),
		},
		Long: heredoc.Doc(`
			Read orb YAML from path, compute the next version by incrementing
			the current latest stable version, and publish it.

			The --bump flag selects which segment to increment. If no stable
			version exists yet, publishes as 0.0.1 (patch), 0.1.0 (minor),
			or 1.0.0 (major) depending on --bump.

			Pass '-' as the path to read from stdin.
		`),
		Example: heredoc.Doc(`
			# Increment patch version and publish
			$ circleci orb publish increment orb.yml myorg/my-orb --bump patch

			# Increment minor version
			$ circleci orb publish increment orb.yml myorg/my-orb --bump minor

			# Read orb from stdin, increment major
			$ cat orb.yml | circleci orb publish increment - myorg/my-orb --bump major
		`),
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.RequireArgs(args, "path", "ns/orb"); err != nil {
				return err
			}
			if !isValidSegment(bump) {
				return clierrors.New("args.invalid_segment", "Invalid semver segment",
					"--bump must be one of: major, minor, patch").
					WithExitCode(clierrors.ExitBadArguments)
			}
			ctx := cmd.Context()
			client, err := cmdutil.LoadClient(ctx)
			if err != nil {
				return err
			}
			return runOrbIncrement(ctx, client, args[0], args[1], bump)
		},
	}

	cmd.Flags().StringVar(&bump, "bump", "", "which version segment to increment: major, minor, or patch")

	return cmd
}

// isValidSegment checks that segment is one of the allowed values.
func isValidSegment(s string) bool {
	return s == "major" || s == "minor" || s == "patch"
}

// parseOrbRef parses "ns/orbname@version" into parts.
func parseOrbRef(ref string) (namespace, orbName, version string, err error) {
	parts := strings.SplitN(ref, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", "", clierrors.New("args.invalid_orb_ref", "Invalid orb reference",
			"Expected <ns>/<orb>@<version>, got: "+ref).
			WithExitCode(clierrors.ExitBadArguments)
	}
	version = parts[1]
	nsOrb := strings.SplitN(parts[0], "/", 2)
	if len(nsOrb) != 2 || nsOrb[0] == "" || nsOrb[1] == "" {
		return "", "", "", clierrors.New("args.invalid_orb_ref", "Invalid orb reference",
			"Expected <ns>/<orb>@<version>, got: "+ref).
			WithExitCode(clierrors.ExitBadArguments)
	}
	return nsOrb[0], nsOrb[1], version, nil
}

func runOrbPublish(ctx context.Context, client *apiclient.Client, path, ref string) error {
	ns, orbName, version, err := parseOrbRef(ref)
	if err != nil {
		return err
	}

	yaml, fileErr := readOrbFile(path)
	if fileErr != nil {
		return clierrors.New("orb.read_error", "Failed to read orb file",
			fileErr.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	pkg, apiErr := client.GetOrbPackageByName(ctx, ns+"/"+orbName)
	if apiErr != nil {
		return orbAPIErr(apiErr, ns+"/"+orbName)
	}

	v, apiErr := client.PublishOrbVersion(ctx, apiclient.PublishOrbVersionRequest{
		OrbID:   pkg.ID,
		YAML:    yaml,
		Version: version,
	})
	if apiErr != nil {
		return orbAPIErr(apiErr, ns+"/"+orbName)
	}

	iostream.Printf(ctx, "%s Published %s@%s\n", iostream.SymbolOK(ctx), v.OrbName, v.Version)
	return nil
}

func runOrbPromote(ctx context.Context, client *apiclient.Client, ref, segment string) error {
	// ref is "ns/orb@dev:label"
	ns, orbName, devVersion, err := parseOrbRef(ref)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(devVersion, "dev:") {
		return clierrors.New("args.invalid_dev_ref", "Invalid dev version reference",
			"Version must start with 'dev:', got: "+devVersion).
			WithExitCode(clierrors.ExitBadArguments)
	}

	// Find the dev version
	pkg, apiErr := client.GetOrbPackageByName(ctx, ns+"/"+orbName)
	if apiErr != nil {
		return orbAPIErr(apiErr, ns+"/"+orbName)
	}

	devRef := ns + "/" + orbName + "@" + devVersion
	devVer, apiErr := client.GetOrbVersionByRef(ctx, devRef)
	if apiErr != nil {
		return orbAPIErr(apiErr, devRef)
	}

	v, apiErr := client.PromoteOrbVersion(ctx, devVer.ID, segment)
	if apiErr != nil {
		return orbAPIErr(apiErr, ref)
	}

	_ = pkg // used above to ensure orb exists
	iostream.Printf(ctx, "%s Promoted %s to %s@%s\n", iostream.SymbolOK(ctx), ref, v.OrbName, v.Version)
	return nil
}

func runOrbIncrement(ctx context.Context, client *apiclient.Client, path, fullName, segment string) error {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return clierrors.New("args.invalid_orb_ref", "Invalid orb name",
			"Expected <ns>/<orb>, got: "+fullName).
			WithExitCode(clierrors.ExitBadArguments)
	}

	yaml, fileErr := readOrbFile(path)
	if fileErr != nil {
		return clierrors.New("orb.read_error", "Failed to read orb file",
			fileErr.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}

	pkg, apiErr := client.GetOrbPackageByName(ctx, fullName)
	if apiErr != nil {
		return orbAPIErr(apiErr, fullName)
	}

	// Get latest stable version
	versions, apiErr := client.ListOrbVersions(ctx, pkg.ID, "stable")
	if apiErr != nil {
		return orbAPIErr(apiErr, fullName)
	}

	var nextVersion string
	if len(versions) == 0 {
		// No stable versions: start at 0.0.1/0.1.0/1.0.0
		switch segment {
		case "major":
			nextVersion = "1.0.0"
		case "minor":
			nextVersion = "0.1.0"
		default:
			nextVersion = "0.0.1"
		}
	} else {
		nextVersion, apiErr = incrementVersion(versions[0].Version, segment)
		if apiErr != nil {
			return apiErr
		}
	}

	v, apiErr := client.PublishOrbVersion(ctx, apiclient.PublishOrbVersionRequest{
		OrbID:   pkg.ID,
		YAML:    yaml,
		Version: nextVersion,
	})
	if apiErr != nil {
		return orbAPIErr(apiErr, fullName)
	}

	iostream.Printf(ctx, "%s Published %s@%s\n", iostream.SymbolOK(ctx), v.OrbName, v.Version)
	return nil
}

// incrementVersion increments a semver string by the given segment.
func incrementVersion(version, segment string) (string, error) {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) != 3 {
		return "", clierrors.New("orb.invalid_version", "Invalid semver version",
			"Expected MAJOR.MINOR.PATCH, got: "+version).
			WithExitCode(clierrors.ExitGeneralError)
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", clierrors.New("orb.invalid_version", "Invalid semver version",
			"Could not parse version components of: "+version).
			WithExitCode(clierrors.ExitGeneralError)
	}
	switch segment {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	default:
		patch++
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}
