package testutil

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type IntegrationTestSuite struct {
	*suite.Suite
	SkipIfShort bool
}

func (its *IntegrationTestSuite) SetupSuite() {
	if its.SkipIfShort {
		if testing.Short() {
			its.T().Skip("skipping integration test in short mode")
		}
	}
}

type BenchmarkSuite struct {
	*suite.Suite
}

func (bs *BenchmarkSuite) RunBenchmark(b *testing.B, name string, fn func(b *testing.B)) {
	b.Run(name, fn)
}

func (bs *BenchmarkSuite) ReportMetric(b *testing.B, name string, value float64) {
	b.ReportMetric(value, name)
}

type TestSuite struct {
	SetupCalled    bool
	TeardownCalled bool
}

func (ts *TestSuite) SetupSuite() {
	ts.SetupCalled = true
}

func (ts *TestSuite) TearDownSuite() {
	ts.TeardownCalled = true
}
