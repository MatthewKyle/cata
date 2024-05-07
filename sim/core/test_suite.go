package core

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/wowsims/cata/sim/core/proto"
	"github.com/wowsims/cata/sim/core/stats"
	"google.golang.org/protobuf/encoding/prototext"
	googleProto "google.golang.org/protobuf/proto"
)

// Precise enough to detect very small changes to test results, but truncated
// enough that we don't have flaky tests due to different OS/Go versions with
// different float rounding behavior.
const storagePrecision = 5

type IndividualTestSuite struct {
	Name string

	// Names of all the tests, in the order they are tested.
	testNames []string

	testResults *proto.TestSuiteResult
}

func NewIndividualTestSuite(suiteName string) *IndividualTestSuite {
	return &IndividualTestSuite{
		Name:        suiteName,
		testResults: newTestSuiteResult(),
	}
}

func (testSuite *IndividualTestSuite) TestCharacterStats(testName string, csr *proto.ComputeStatsRequest) {
	testSuite.testNames = append(testSuite.testNames, testName)

	result := ComputeStats(csr)
	finalStats := stats.FromFloatArray(result.RaidStats.Parties[0].Players[0].FinalStats.Stats)

	testSuite.testResults.CharacterStatsResults[testName] = &proto.CharacterStatsTestResult{
		FinalStats: toFixedStats(finalStats[:], storagePrecision),
	}
}

func (testSuite *IndividualTestSuite) TestStatWeights(testName string, swr *proto.StatWeightsRequest) {
	testSuite.testNames = append(testSuite.testNames, testName)

	result := StatWeights(swr)
	weights := stats.FromFloatArray(result.Dps.Weights.Stats)

	testSuite.testResults.StatWeightsResults[testName] = &proto.StatWeightsTestResult{
		Weights: toFixedStats(weights[:], storagePrecision),
	}
}

func (testSuite *IndividualTestSuite) TestDPS(testName string, rsr *proto.RaidSimRequest) {
	testSuite.testNames = append(testSuite.testNames, testName)

	result := RunRaidSim(rsr)
	if result.Logs != "" {
		fmt.Printf("LOGS: %s\n", result.Logs)
	}
	if result.ErrorResult != "" {
		panic("simulation failed to run: " + result.ErrorResult)
	}
	testSuite.testResults.DpsResults[testName] = &proto.DpsTestResult{
		Dps:  toFixed(result.RaidMetrics.Dps.Avg, storagePrecision),
		Tps:  toFixed(result.RaidMetrics.Parties[0].Players[0].Threat.Avg, storagePrecision),
		Dtps: toFixed(result.RaidMetrics.Parties[0].Players[0].Dtps.Avg, storagePrecision),
		Hps:  toFixed(result.RaidMetrics.Parties[0].Players[0].Hps.Avg, storagePrecision),
	}
}

type ResetTestResult struct {
	BaseDps   float64
	BaseHps   float64
	BaseTps   float64
	BaseDtps  float64
	SplitDps  float64
	SplitHps  float64
	SplitTps  float64
	SplitDtps float64
}

// The purpose of this test is to check if the sim resets everything properly between iterations.
// It runs one normal sim with totalIterations, and then splits totalIterations over multiple sims and combines the results.
// If everything works then both the "base" and "split" results should be the same.
func (testSuite *IndividualTestSuite) TestResetLeakage(testName string, rsr *proto.RaidSimRequest, totalIterations int32, splits int32) ResetTestResult {
	testSuite.testNames = append(testSuite.testNames, testName)

	rsrCopy := googleProto.Clone(rsr).(*proto.RaidSimRequest)
	results := ResetTestResult{}

	rsrCopy.SimOptions.Iterations = totalIterations
	resultBase := RunRaidSim(rsrCopy)
	results.BaseDps = resultBase.RaidMetrics.Dps.Avg
	results.BaseTps = resultBase.RaidMetrics.Parties[0].Players[0].Threat.Avg
	results.BaseDtps = resultBase.RaidMetrics.Parties[0].Players[0].Dtps.Avg
	results.BaseHps = resultBase.RaidMetrics.Parties[0].Players[0].Hps.Avg

	nextStartSeed := rsrCopy.SimOptions.RandomSeed
	for i := 0; i < int(splits); i++ {
		rsrCopy.SimOptions.Iterations = totalIterations / splits
		if i == 0 {
			rsrCopy.SimOptions.Iterations += totalIterations % splits
		}
		rsrCopy.SimOptions.RandomSeed = nextStartSeed
		nextStartSeed += int64(rsrCopy.SimOptions.Iterations)

		resultSplit := RunRaidSim(rsrCopy)

		weight := float64(rsrCopy.SimOptions.Iterations) / float64(totalIterations)
		results.SplitDps += resultSplit.RaidMetrics.Dps.Avg * weight
		results.SplitTps += resultSplit.RaidMetrics.Parties[0].Players[0].Threat.Avg * weight
		results.SplitDtps += resultSplit.RaidMetrics.Parties[0].Players[0].Dtps.Avg * weight
		results.SplitHps += resultSplit.RaidMetrics.Parties[0].Players[0].Hps.Avg * weight
	}

	return results
}

func (testSuite *IndividualTestSuite) TestCasts(testName string, rsr *proto.RaidSimRequest) {
	testSuite.testNames = append(testSuite.testNames, testName)
	result := RunRaidSim(rsr)
	if result.Logs != "" {
		fmt.Printf("LOGS: %s\n", result.Logs)
	}
	if result.ErrorResult != "" {
		panic("simulation failed to run: " + result.ErrorResult)
	}
	castsByAction := make(map[string]float64, 0)
	for _, metric := range result.RaidMetrics.Parties[0].Players[0].Actions {
		name := metric.Id.String()
		name = strings.ReplaceAll(name, "  ", " ")
		for _, targetMetrics := range metric.Targets {
			if val, ok := castsByAction[name]; ok {
				castsByAction[name] = val + float64(targetMetrics.Casts)
			} else {
				castsByAction[name] = float64(targetMetrics.Casts)
			}
		}
		castsByAction[name] /= float64(rsr.SimOptions.Iterations)
		castsByAction[name] *= 10
		castsByAction[name] = math.Round(castsByAction[name]) / 10.0
	}
	casts := &proto.CastsTestResult{Casts: castsByAction}
	testSuite.testResults.CastsResults[testName] = casts
}

func (testSuite *IndividualTestSuite) Done(t *testing.T) {
	testSuite.writeToFile()
}

const tolerance = 0.00001

func (testSuite *IndividualTestSuite) writeToFile() {
	str := prototext.Format(testSuite.testResults)
	// For some reason the formatter sometimes outputs 2 spaces instead of one.
	// Replace so we get consistent output.
	str = strings.ReplaceAll(str, "  ", " ")
	data := []byte(str)

	err := os.WriteFile(testSuite.Name+".results.tmp", data, 0644)
	if err != nil {
		panic(err)
	}
}

func (testSuite *IndividualTestSuite) readExpectedResults() (*proto.TestSuiteResult, error) {
	data, err := os.ReadFile(testSuite.Name + ".results")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newTestSuiteResult(), nil
		}
		return nil, err
	}

	results := &proto.TestSuiteResult{}
	if err = prototext.Unmarshal(data, results); err != nil {
		return nil, err
	}
	return results, err
}

func newTestSuiteResult() *proto.TestSuiteResult {
	return &proto.TestSuiteResult{
		CharacterStatsResults: make(map[string]*proto.CharacterStatsTestResult),
		StatWeightsResults:    make(map[string]*proto.StatWeightsTestResult),
		DpsResults:            make(map[string]*proto.DpsTestResult),
		CastsResults:          make(map[string]*proto.CastsTestResult),
	}
}

type TestGenerator interface {
	// The total number of tests that this generator can generate.
	NumTests() int

	// The name and API request for the test with the given index.
	GetTest(testIdx int) (string, *proto.ComputeStatsRequest, *proto.StatWeightsRequest, *proto.RaidSimRequest)
}

func RunTestSuite(t *testing.T, suiteName string, generator TestGenerator) {
	testSuite := NewIndividualTestSuite(suiteName)
	var currentTestName string

	defer func() {
		if p := recover(); p != nil {
			panic(fmt.Sprintf("Panic during test %s: %v", currentTestName, p))
		}
	}()

	expectedResults, err := testSuite.readExpectedResults()
	if err != nil {
		t.Logf("\n\n----- FAILURE LOADING RESULTS FILE TESTS WILL FAIL-----\n%s\n-----\n\n", err)
		t.Fail()
	}

	numTests := generator.NumTests()
	for i := 0; i < numTests; i++ {
		testName, csr, swr, rsr := generator.GetTest(i)
		if strings.Contains(testName, "Average") && testing.Short() {
			continue
		}
		currentTestName = testName

		t.Run(currentTestName, func(t *testing.T) {
			fullTestName := suiteName + "-" + testName
			if csr != nil {
				testSuite.TestCharacterStats(fullTestName, csr)
				if actualCharacterStats, ok := testSuite.testResults.CharacterStatsResults[fullTestName]; ok {
					actualStats := stats.FromFloatArray(actualCharacterStats.FinalStats)
					if expectedCharacterStats, ok := expectedResults.CharacterStatsResults[fullTestName]; ok {
						expectedStats := stats.FromFloatArray(expectedCharacterStats.FinalStats)
						if !actualStats.EqualsWithTolerance(expectedStats, tolerance) {
							t.Logf("Stats expected %v but was %v", expectedStats, actualStats)
							t.Fail()
						}
					} else {
						t.Logf("Unexpected test %s with stats: %v", fullTestName, actualStats)
						t.Fail()
					}
				} else if !ok {
					t.Logf("Missing Result for test %s", fullTestName)
					t.Fail()
				}
			} else if swr != nil {
				testSuite.TestStatWeights(fullTestName, swr)
				if actualStatWeights, ok := testSuite.testResults.StatWeightsResults[fullTestName]; ok {
					actualWeights := stats.FromFloatArray(actualStatWeights.Weights)
					if expectedStatWeights, ok := expectedResults.StatWeightsResults[fullTestName]; ok {
						expectedWeights := stats.FromFloatArray(expectedStatWeights.Weights)
						if !actualWeights.EqualsWithTolerance(expectedWeights, tolerance) {
							t.Logf("Weights expected %v but was %v", expectedWeights, actualWeights)
							t.Fail()
						}
					} else {
						t.Logf("Unexpected test %s with stat weights: %v", fullTestName, actualWeights)
						t.Fail()
					}
				} else if !ok {
					t.Logf("Missing Result for test %s", fullTestName)
					t.Fail()
				}
			} else if rsr != nil && !strings.Contains(testName, "Casts") {
				testSuite.TestDPS(fullTestName, rsr)
				if actualDpsResult, ok := testSuite.testResults.DpsResults[fullTestName]; ok {
					if expectedDpsResult, ok := expectedResults.DpsResults[fullTestName]; ok {
						// Check whichever of DPS/HPS is larger first, so we get better test diff printouts.
						if actualDpsResult.Dps < actualDpsResult.Hps {
							if actualDpsResult.Hps < expectedDpsResult.Hps-tolerance || actualDpsResult.Hps > expectedDpsResult.Hps+tolerance {
								t.Logf("HPS expected %0.03f but was %0.03f!.", expectedDpsResult.Hps, actualDpsResult.Hps)
								t.Fail()
							}
						}
						if actualDpsResult.Dps < expectedDpsResult.Dps-tolerance || actualDpsResult.Dps > expectedDpsResult.Dps+tolerance {
							t.Logf("DPS expected %0.03f but was %0.03f!.", expectedDpsResult.Dps, actualDpsResult.Dps)
							t.Fail()
						}
						if actualDpsResult.Dps >= actualDpsResult.Hps {
							if actualDpsResult.Hps < expectedDpsResult.Hps-tolerance || actualDpsResult.Hps > expectedDpsResult.Hps+tolerance {
								t.Logf("HPS expected %0.03f but was %0.03f!.", expectedDpsResult.Hps, actualDpsResult.Hps)
								t.Fail()
							}
						}

						if actualDpsResult.Tps < expectedDpsResult.Tps-tolerance || actualDpsResult.Tps > expectedDpsResult.Tps+tolerance {
							t.Logf("TPS expected %0.03f but was %0.03f!.", expectedDpsResult.Tps, actualDpsResult.Tps)
							t.Fail()
						}
						if actualDpsResult.Dtps < expectedDpsResult.Dtps-tolerance || actualDpsResult.Dtps > expectedDpsResult.Dtps+tolerance {
							t.Logf("DTPS expected %0.03f but was %0.03f!.", expectedDpsResult.Dtps, actualDpsResult.Dtps)
							t.Fail()
						}
					} else {
						t.Logf("Unexpected test %s with %0.03f DPS!", fullTestName, actualDpsResult.Dps)
						t.Fail()
					}
				} else if !ok {
					t.Logf("Missing Result for test %s", fullTestName)
					t.Fail()
				}
			} else if rsr != nil && strings.Contains(testName, "Casts") {
				testSuite.TestCasts(fullTestName, rsr)
				if actualCastsResult, ok := testSuite.testResults.CastsResults[fullTestName]; ok {
					if expectedCastsResult, ok := expectedResults.CastsResults[fullTestName]; ok {
						for action, casts := range actualCastsResult.Casts {
							if casts < expectedCastsResult.Casts[action]-tolerance || casts > expectedCastsResult.Casts[action]+tolerance {
								t.Logf("Expected %0.03f casts of %s but was %0.03f!.", expectedCastsResult.Casts[action], action, casts)
								t.Fail()
							}
						}
					} else {
						t.Logf("Unexpected test %s", fullTestName)
						t.Fail()
					}
				} else if !ok {
					t.Logf("Missing Result for test %s", fullTestName)
					t.Fail()
				}
			} else {
				panic("No test request provided")
			}
		})
		if rsr != nil {
			resetTestName := suiteName + "-" + testName + "/ResetTest"
			t.Run(resetTestName, func(t *testing.T) {
				setups := [][]int32{
					{4, 2},
					{20, 3},
				}
				for _, setup := range setups {
					res := testSuite.TestResetLeakage(resetTestName, rsr, setup[0], setup[1])
					if math.Abs(res.BaseDps-res.SplitDps) > tolerance {
						t.Logf("DPS did not match! Base was %0.03f and split was %0.03f!. Something probably doesn't reset correctly on sim reset!", res.BaseDps, res.SplitDps)
						t.Fail()
					}
					if math.Abs(res.BaseHps-res.SplitHps) > tolerance {
						t.Logf("HPS did not match! Base was %0.03f and split was %0.03f!. Something probably doesn't reset correctly on sim reset!", res.BaseHps, res.SplitHps)
						t.Fail()
					}
					if math.Abs(res.BaseTps-res.SplitTps) > tolerance {
						t.Logf("TPS did not match! Base was %0.03f and split was %0.03f!. Something probably doesn't reset correctly on sim reset!", res.BaseTps, res.SplitTps)
						t.Fail()
					}
					if math.Abs(res.BaseDtps-res.SplitDtps) > tolerance {
						t.Logf("DTPS did not match! Base was %0.03f and split was %0.03f!. Something probably doesn't reset correctly on sim reset!", res.BaseDtps, res.SplitDtps)
						t.Fail()
					}
				}
			})
		}
	}

	testSuite.Done(t)

	if t.Failed() {
		t.Log("One or more tests failed! If the changes are intentional, update the expected results with 'make test && make update-tests'. Otherwise go fix your bugs!")
	}
}

func round(num float64) int {
	return int(num + math.Copysign(0.5, num))
}

func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(round(num*output)) / output
}

func toFixedStats(s []float64, precision int) []float64 {
	for i, val := range s {
		s[i] = toFixed(val, precision)
	}
	return s
}
