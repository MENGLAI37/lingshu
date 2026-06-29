package testutil

import (
"testing"

"github.com/stretchr/testify/suite"
)

// ===========================================================================
// IntegrationTestSuite - Suite for integration tests
// ===========================================================================

// IntegrationTestSuite provides a base for integration tests.
// It directly embeds suite.Suite to ensure methods like T() are available.
type IntegrationTestSuite struct {
*suite.Suite // 👈 直接嵌入，这是解决报错的关键
SkipIfShort bool
}

// SetupSuite skips integration tests when running with -short flag.
func (its *IntegrationTestSuite) SetupSuite() {
// its.T() 现在可以直接使用了
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
*suite.Suite
}

// RunBenchmark runs a benchmark test with the given name and function.
func (bs *BenchmarkSuite) RunBenchmark(b *testing.B, name string, fn func(b *testing.B)) {
b.Run(name, fn)
}

// ReportMetric reports a custom metric for the benchmark.
func (bs *BenchmarkSuite) ReportMetric(b *testing.B, name string, value float64) {
b.ReportMetric(value, name)
}

// ===========================================================================
// TestSuite - Base test suite (Optional)
// ===========================================================================

// 如果你还有其他通用逻辑，可以保留这个结构体，但建议直接使用 IntegrationTestSuite
// 或者将其改为非嵌入式结构，通过方法接收者来调用。

// TestSuite is a base test suite providing common testing utilities.
// 注意：这里不再嵌入 *suite.Suite，避免多重嵌入的歧义
type TestSuite struct {
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