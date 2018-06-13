package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCircleciCli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CircleciCli Suite")
}
