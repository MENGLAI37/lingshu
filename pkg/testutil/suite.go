package testutil

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// ===========================================================================
// TestSuite - Base test suite with common setup/teardown
// ===========================================================================

// TestSuite is a base test suite providing common testing utilities.
type TestSuite struct {
	suite.Suite
	SetupCalled    bool
	TeardownCalled bool
}

// SetupSuite is called once before running the test suite.
func (ts *TestSuite) SetupSuite() {
	ts.SetupCalled = true
}

// TearDownSuite is called once after running the test suite.
func (ts *TestSuite) TearDownSuite() {
	ts.TeardownCalled = true
}

// Run runs the test suite.
func (ts *TestSuite) Run(t *testing.T) {
	suite.Run(t, ts)
}

// ===========================================================================
// IntegrationTestSuite - Suite for integration tests
// ===========================================================================

// IntegrationTestSuite provides a base for integration tests.
type IntegrationTestSuite struct {
	TestSuite
	SkipIfShort bool
}

// SetupSuite skips integration tests when running with -short flag.
func (its *IntegrationTestSuite) SetupSuite() {
	its.TestSuite.SetupSuite()
	if its.SkipIfShort {
		if testing.Short() {
			its.T().Skip("skipping integration test in short mode")
		}
	}
}

// ===========================================================================
// BenchmarkSuite - Suite for benchmarks
// ===========================================================================

// BenchmarkSuite provides utilities for running benchmarks.
type BenchmarkSuite struct {
	suite.Suite
}

// RunBenchmark runs a benchmark test with the given name and function.
func (bs *BenchmarkSuite) RunBenchmark(b *testing.B, name string, fn func(b *testing.B)) {
	b.Run(name, fn)
}

// ReportMetric reports a custom metric for the benchmark.
func (bs *BenchmarkSuite) ReportMetric(b *testing.B, name string, value float64) {
	b.ReportMetric(value, name)
}
