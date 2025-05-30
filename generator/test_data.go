package generator

import (
	"fmt"
	"io/ioutil"
	"sort"
	"time"

	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	tmmath "github.com/cometbft/cometbft/libs/math"

	"github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/light/provider"
	"github.com/cometbft/cometbft/types"
)

var (
	genTime, _         = time.Parse(time.RFC3339, "2019-11-02T15:04:00Z")
	now, _             = time.Parse(time.RFC3339, "2019-11-02T15:30:00Z")
	firstBlockTime, _  = time.Parse(time.RFC3339, "2019-11-02T15:04:10Z")
	secondBlockTime, _ = time.Parse(time.RFC3339, "2019-11-02T15:04:15Z")
	thirdBlockTime, _  = time.Parse(time.RFC3339, "2019-11-02T15:04:20Z")
)

const (
	TRUSTING_PERIOD       = 3 * time.Hour
	expectedOutputNoError = "no error"
	expectedOutputError   = "error"
)

// SINGLE STEP TEST DATA -------->

// TestCase stores all the necessary information for single step test cases
// to perform verification test on the data given
type TestCase struct {
	Description    string      `json:"description"`
	Initial        Initial     `json:"initial"`
	Input          []LiteBlock `json:"input"`
	ExpectedOutput string      `json:"expected_output"`
}

// genJSON produces the JSON for the given testCase type.
// The ouput is saved under the specified file parameter
// TODO: rename to - intoJSON
func (tc TestCase) genJSON(file string) {

	json.RegisterInterface((*types.Evidence)(nil), nil)

	b, err := json.MarshalJSONIndent(tc, " ", "	")
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	_ = ioutil.WriteFile(file, b, 0644)

}

// makeTestCase copies over the given parameters to the TestCase struct and returns it
func makeTestCase(description string, initial Initial, input []LiteBlock, expectedOutput string) TestCase {
	return TestCase{
		Description:    description,
		Initial:        initial,
		Input:          input,
		ExpectedOutput: expectedOutput,
	}
}

// LiteBlock refers to the minimum data a lite client interacts with.
// Essentially, it only requires a SignedHeader and Validator Set for current and next height
type LiteBlock struct {
	SignedHeader     types.SignedHeader `json:"signed_header"`
	ValidatorSet     types.ValidatorSet `json:"validator_set"`
	NextValidatorSet types.ValidatorSet `json:"next_validator_set"`
}

// Initial stores the data required by a test case to set the context
// i.e. the initial state to begin the test from
type Initial struct {
	SignedHeader     types.SignedHeader `json:"signed_header"`
	NextValidatorSet types.ValidatorSet `json:"next_validator_set"`
	TrustingPeriod   time.Duration      `json:"trusting_period"`
	Now              time.Time          `json:"now"`
}

// ValList stores a list of validators and privVals
// It is populated from the lite-client/tests/json/val_list.json
// It used to have a predefined set of validators for mocking the test data
type ValList struct {
	Validators []*types.Validator            `json:"validators"`
	PrivVals   types.PrivValidatorsByAddress `json:"priv_val"`
}

// Copy is essentially used to dereference the pointer
// ValList contains valSet pointer and privVal interface
// So to avoid manipulating the original list, we better copy it!
func (valList ValList) Copy() (vl ValList) {

	for _, val := range valList.Validators {
		copyVal := *val
		vl.Validators = append(vl.Validators, &copyVal)
	}

	for _, privVal := range valList.PrivVals {
		copyPrivVal := privVal
		vl.PrivVals = append(vl.PrivVals, copyPrivVal)
	}
	return
}

// GetValList reads the validators and privals list in the file
// unmarshals it to ValList struct
// "file" parameter specifies the path to the val_list.json file
func GetValList(file string) ValList {
	data := ReadFile(file)
	var valList ValList

	json.RegisterInterface((*types.PrivValidator)(nil), nil)
	json.RegisterConcrete(&types.MockPV{}, "tendermint/MockPV", nil)

	er := json.UnmarshalJSON(data, &valList)
	if er != nil {
		fmt.Printf("error: %v", er)
	}

	return valList
}

// GenerateValList produces a val_list.json file which contains a list validators and privVals
// of given number abd voting power
func GenerateValList(numVals int, votingPower int64) {

	valSet, newPrivVal := types.RandValidatorSet(numVals, votingPower)
	sort.Sort(types.ValidatorsByAddress(valSet.Validators))
	valList := &ValList{
		Validators: valSet.Validators,
		PrivVals:   types.PrivValidatorsByAddress(newPrivVal),
	}

	json.RegisterInterface((*types.PrivValidator)(nil), nil)
	json.RegisterConcrete(&types.MockPV{}, "tendermint/MockPV", nil)

	b, err := json.MarshalJSONIndent(valList, " ", "	")

	if err != nil {
		panic(err)
	}

	file := "./val_list.json"
	err = ioutil.WriteFile(file, b, 0644)
	if err != nil {
		panic(err)
	}
}

func generateTxs() []types.Tx {
	// Empty txs
	return []types.Tx{}
}

func generateEvidences() []types.Evidence {
	// Empty evidences
	return []types.Evidence{}
}

func newAbsentCommitSig(valAddr types.Address) types.CommitSig {
	return types.CommitSig{
		BlockIDFlag: types.BlockIDFlagAbsent,
		// According to the spec an absent CommitSig is expected to have a ValidatorAddress
		// But the Go implementation isn't following that currently
		// So, for now, we let it be as it is in Go code

		// ValidatorAddress: valAddr,
	}
}

// BISECTION TEST DATA -------->

type TestBisection struct {
	Description        string              `json:"description"`
	TrustOptions       TrustOptions        `json:"trust_options"`
	Primary            MockProvider        `json:"primary"`
	Witnesses          []provider.Provider `json:"witnesses"`
	HeightToVerify     int64               `json:"height_to_verify"`
	Now                time.Time           `json:"now"`
	ExpectedOutput     string              `json:"expected_output"`
	ExpectedBisections int32               `json:"expected_num_of_bisections"`
}

func (tb TestBisection) make(
	desc string,
	trustOpts TrustOptions,
	primary MockProvider,
	witnesses []provider.Provider,
	heightToVerify int64,
	now time.Time,
	expectedOutput string,
	expectedBisections int32,
) TestBisection {
	return TestBisection{
		Description:        desc,
		TrustOptions:       trustOpts,
		Primary:            primary,
		Witnesses:          witnesses,
		HeightToVerify:     heightToVerify,
		Now:                now,
		ExpectedOutput:     expectedOutput,
		ExpectedBisections: expectedBisections,
	}
}

func (testBisection TestBisection) genJSON(file string) {
	json.RegisterInterface((*types.Evidence)(nil), nil)
	json.RegisterInterface((*provider.Provider)(nil), nil)
	json.RegisterConcrete(MockProvider{}, "com.tendermint/MockProvider", nil)

	b, err := json.MarshalJSONIndent(testBisection, " ", "	")
	if err != nil {
		fmt.Printf("error: %v", err)
	}

	_ = ioutil.WriteFile(file, b, 0644)
}

type TrustOptions struct {
	// Trusting Period
	Period time.Duration `json:"period"`
	// Trusted Header Height
	Height int64 `json:"height"`
	// Trusted Header Hash
	Hash       tmbytes.HexBytes `json:"hash"`
	TrustLevel tmmath.Fraction  `json:"trust_level"`
}

func (t TrustOptions) make(
	sh types.SignedHeader,
	trustingPeriod time.Duration,
	trustLevel tmmath.Fraction,
) TrustOptions {
	return TrustOptions{
		Period:     trustingPeriod,
		Height:     sh.Header.Height,
		Hash:       sh.Commit.BlockID.Hash,
		TrustLevel: trustLevel,
	}

}

type MockProvider struct {
	ChainId    string      `json:"chain_id"`
	LiteBlocks []LiteBlock `json:"lite_blocks"`
}

func (mp MockProvider) New(chainID string, liteBlocks []LiteBlock) MockProvider {
	return MockProvider{
		ChainId:    chainID,
		LiteBlocks: liteBlocks,
	}
}

func (mp MockProvider) Copy() MockProvider {
	return MockProvider{
		ChainId:    mp.ChainId,
		LiteBlocks: mp.LiteBlocks,
	}
}

func (mp MockProvider) ChainID() string {
	return mp.ChainId
}

func (mp MockProvider) SignedHeader(height int64) (*types.SignedHeader, error) {
	fmt.Printf("\n sh -- req h: %v", height)
	for _, lb := range mp.LiteBlocks {
		if lb.SignedHeader.Header.Height == height {
			return &lb.SignedHeader, nil
		}
	}
	return nil, provider.ErrSignedHeaderNotFound
}
func (mp MockProvider) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	fmt.Printf("\n vs -- req h: %v", height)
	// if lb.SignedHeader.Header.Height+1 == height {
	// 		return &lb.NextValidatorSet, nil
	// 	}
	for _, lb := range mp.LiteBlocks {
		if lb.SignedHeader.Header.Height == height {
			return &lb.ValidatorSet, nil
		}
	}
	return nil, provider.ErrValidatorSetNotFound
}

type ValSetChanges []ValList

func (vsc ValSetChanges) getDefault(valList ValList) ValSetChanges {
	valsArray := [][]*types.Validator{
		valList.Validators[:2],
		valList.Validators[:2],
		valList.Validators[:2],
		valList.Validators[:2],
		valList.Validators[:2],
		valList.Validators[3:5],
		valList.Validators[3:5],
		valList.Validators[3:5],
		valList.Validators[3:5],
		valList.Validators[3:5],
		valList.Validators[3:5],
	}
	privValsArray := []types.PrivValidatorsByAddress{
		valList.PrivVals[:2],
		valList.PrivVals[:2],
		valList.PrivVals[:2],
		valList.PrivVals[:2],
		valList.PrivVals[:2],
		valList.PrivVals[3:5],
		valList.PrivVals[3:5],
		valList.PrivVals[3:5],
		valList.PrivVals[3:5],
		valList.PrivVals[3:5],
		valList.PrivVals[3:5],
	}
	return vsc.makeValSetChanges(valsArray, privValsArray)
}

func (vsc ValSetChanges) makeValSetChangeAtHeight(
	height int,
	vals []*types.Validator,
	privVals types.PrivValidatorsByAddress,
) ValSetChanges {
	vsc[height] = ValList{
		Validators: vals,
		PrivVals:   privVals,
	}
	return vsc
}

func (vsc ValSetChanges) makeValSetChanges(
	vals [][]*types.Validator,
	privVals []types.PrivValidatorsByAddress,
) ValSetChanges {
	for i := range vals {
		v := ValList{
			Validators: vals[i],
			PrivVals:   privVals[i],
		}
		vsc = append(vsc, v)
	}
	return vsc
}
