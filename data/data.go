package data

import (
	packr "github.com/gobuffalo/packr/v2"
	yaml "gopkg.in/yaml.v2"
)

// DataYML maps the YAML found in _data/data.yml
// Be sure to update this type when you modify the structure of that file!
type DataYML struct {
	Links struct {
		CLIDocs     string `yaml:"cli_docs"`
		OrbDocs     string `yaml:"orb_docs"`
		NewAPIToken string `yaml:"new_api_token"`
	} `yaml:"links"`
}

// LoadData should be called once to decode the YAML into DataYML.
func LoadData() (*DataYML, error) {
	var (
		bts []byte
		err error
	)

	d := &DataYML{}
	box := packr.New("circleci-cli-box", "../_data")

	bts, err = box.Find("data.yml")
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bts, &d)
	if err != nil {
		return nil, err
	}

	return d, nil
}
