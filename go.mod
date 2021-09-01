module github.com/CircleCI-Public/circleci-cli

require (
	github.com/AlecAivazis/survey/v2 v2.1.1
	github.com/Masterminds/semver v1.4.2
	github.com/blang/semver v3.5.1+incompatible
	github.com/briandowns/spinner v0.0.0-20181018151057-dd69c579ff20
	github.com/fatih/color v1.9.0 // indirect
	github.com/go-git/go-git/v5 v5.1.0
	github.com/gobuffalo/buffalo-plugins v1.9.3 // indirect
	github.com/gobuffalo/flect v0.0.0-20181210151238-24a2b68e0316 // indirect
	github.com/gobuffalo/packr/v2 v2.0.0-rc.13
	github.com/google/go-github v15.0.0+incompatible // indirect
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135 // indirect
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mitchellh/mapstructure v1.1.2
	github.com/olekukonko/tablewriter v0.0.4
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.4
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/pkg/errors v0.8.1
	github.com/rhysd/go-github-selfupdate v0.0.0-20180520142321-41c1bbb0804a
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/tcnksm/go-gitconfig v0.1.2 // indirect
	github.com/ulikunitz/xz v0.5.9 // indirect
	golang.org/x/oauth2 v0.0.0-20180724155351-3d292e4d0cdc // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c
	gotest.tools/v3 v3.0.2
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/fsnotify/fsnotify v1.4.7 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.0.0 // indirect
	github.com/gobuffalo/envy v1.6.11 // indirect
	github.com/gobuffalo/events v1.1.8 // indirect
	github.com/gobuffalo/genny v0.0.0-20181211165820-e26c8466f14d // indirect
	github.com/gobuffalo/logger v0.0.0-20181127160119-5b956e21995c // indirect
	github.com/gobuffalo/mapi v1.0.1 // indirect
	github.com/gobuffalo/meta v0.0.0-20181127070345-0d7e59dd540b // indirect
	github.com/gobuffalo/packd v0.0.0-20181212173646-eca3b8fd6687 // indirect
	github.com/gobuffalo/syncx v0.0.0-20181120194010-558ac7de985f // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/joho/godotenv v1.3.0 // indirect
	github.com/karrick/godirwalk v1.7.7 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190725054713-01f96b0aa0cd // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.1 // indirect
	github.com/markbates/oncer v0.0.0-20181203154359-bf2de49a0be2 // indirect
	github.com/markbates/safe v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/nxadm/tail v1.4.4 // indirect
	github.com/rogpeppe/go-internal v1.0.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/sirupsen/logrus v1.2.0 // indirect
	github.com/xanzy/ssh-agent v0.2.1 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9 // indirect
	golang.org/x/net v0.0.0-20201202161906-c7110b5ffcbb // indirect
	golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/tools v0.0.0-20190624222133-a101b041ded4 // indirect
	golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543 // indirect
	google.golang.org/protobuf v1.23.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
)

// fix vulnerability: CVE-2020-15114 in etcd v3.3.10+incompatible
replace github.com/coreos/etcd => github.com/coreos/etcd v3.3.24+incompatible

go 1.17
