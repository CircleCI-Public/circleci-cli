module github.com/CircleCI-Public/circleci-cli

require (
	github.com/AlecAivazis/survey/v2 v2.3.4
	github.com/Masterminds/semver v1.4.2
	github.com/blang/semver v3.5.1+incompatible
	github.com/briandowns/spinner v0.0.0-20181018151057-dd69c579ff20
	github.com/fatih/color v1.9.0 // indirect
	github.com/go-git/go-git/v5 v5.1.0
	github.com/google/go-github v15.0.0+incompatible // indirect
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135 // indirect
	github.com/google/uuid v1.3.0
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mitchellh/mapstructure v1.1.2
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.4
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pkg/errors v0.9.1
	github.com/rhysd/go-github-selfupdate v0.0.0-20180520142321-41c1bbb0804a
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/tcnksm/go-gitconfig v0.1.2 // indirect
	github.com/ulikunitz/xz v0.5.9 // indirect
	golang.org/x/oauth2 v0.0.0-20180724155351-3d292e4d0cdc // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c
	gotest.tools/v3 v3.0.2
)

require (
	github.com/charmbracelet/lipgloss v0.5.0
	github.com/elewis787/boa v0.1.1
	github.com/erikgeiser/promptkit v0.6.0
)

require (
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/charmbracelet/bubbles v0.10.3 // indirect
	github.com/charmbracelet/bubbletea v0.20.0 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.0.0 // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190725054713-01f96b0aa0cd // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/muesli/ansi v0.0.0-20211031195517-c9f0611b6c70 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.11.1-0.20220212125758-44cd13922739 // indirect
	github.com/nxadm/tail v1.4.4 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/sahilm/fuzzy v0.1.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/xanzy/ssh-agent v0.2.1 // indirect
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb // indirect
	golang.org/x/sys v0.0.0-20220503163025-988cb79eb6c6 // indirect
	golang.org/x/term v0.0.0-20220411215600-e5f449aeb171 // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 // indirect
	google.golang.org/protobuf v1.23.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

// fix vulnerability: CVE-2020-15114 in etcd v3.3.10+incompatible
replace github.com/coreos/etcd => github.com/coreos/etcd v3.3.24+incompatible

go 1.17
