module github.com/CircleCI-Public/circleci-cli

require (
	github.com/AlecAivazis/survey/v2 v2.1.1
	github.com/Masterminds/semver v1.4.2
	github.com/blang/semver v3.5.1+incompatible
	github.com/briandowns/spinner v0.0.0-20181018151057-dd69c579ff20
	github.com/fatih/color v1.9.0 // indirect
	github.com/go-git/go-git/v5 v5.1.0
	github.com/google/go-github v15.0.0+incompatible // indirect
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135 // indirect
	github.com/google/uuid v1.3.0
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

require golang.org/x/sys v0.0.0-20220502124256-b6088ccd6cba // indirect

// fix vulnerability: CVE-2020-15114 in etcd v3.3.10+incompatible
replace github.com/coreos/etcd => github.com/coreos/etcd v3.3.24+incompatible

go 1.17
