package generator

import (
	"fmt"
	"time"

	lite "github.com/cometbft/cometbft/light"
	st "github.com/cometbft/cometbft/state"

	"github.com/cometbft/cometbft/light/provider"
	"github.com/cometbft/cometbft/types"
)

func generateNextBlocks(
	numOfBlocks int,
	state st.State,
	privVals types.PrivValidatorsByAddress,
	lastCommit *types.Commit,
	valSetChanges ValSetChanges,
	blockTime time.Time,
) ([]LiteBlock, []st.State, types.PrivValidatorsByAddress) {
	var liteBlocks []LiteBlock
	var states []st.State
	valSetChanges = append(valSetChanges, valSetChanges[len(valSetChanges)-1])
	for i := 0; i < numOfBlocks; i++ {
		liteblock, st, _ := generateNextBlockWithNextValsUpdate(
			state,
			valSetChanges[i].PrivVals,
			lastCommit,
			valSetChanges[i+1].Validators,
			nil,
			blockTime,
		)
		liteBlocks = append(liteBlocks, liteblock)
		state = st
		lastCommit = liteblock.SignedHeader.Commit
		states = append(states, state)
		blockTime = blockTime.Add(5 * time.Second)
	}
	return liteBlocks, states, privVals
}

func makeLiteblocks(
	valSetChanges ValSetChanges,
) ([]LiteBlock, []st.State, types.PrivValidatorsByAddress) {
	signedHeader, state, _ := generateFirstBlockWithNextValsUpdate(
		valSetChanges[0].Validators,
		valSetChanges[0].PrivVals,
		valSetChanges[1].Validators,
		nil,
		firstBlockTime,
	)

	firstBlock := []LiteBlock{
		{
			SignedHeader:     signedHeader,
			ValidatorSet:     *state.LastValidators,
			NextValidatorSet: *state.Validators,
		},
	}
	lastCommit := signedHeader.Commit
	numOfBlocks := len(valSetChanges) - 1
	liteBlocks, states, privVals := generateNextBlocks(
		numOfBlocks,
		state,
		valSetChanges[1].PrivVals,
		lastCommit,
		valSetChanges[1:],
		thirdBlockTime,
	)
	liteBlocks = append(firstBlock, liteBlocks...)
	stateSlice := []st.State{
		state,
	}
	states = append(stateSlice, states...)
	return liteBlocks, states, privVals
}

func generateMultiPeerBisectionCase(
	description string,
	primaryValSetChanges ValSetChanges,
	alternativeValSetChanges ValSetChanges,
	expectedBisections int32,
	expectOutput string,
) (TestBisection, []st.State, types.PrivValidatorsByAddress, []st.State, types.PrivValidatorsByAddress) {
	testBisection, statesPrimary, privValsPrimary := generateGeneralBisectionCase(
		description,
		primaryValSetChanges,
		expectedBisections)

	liteBlocks, statesAlternative, privValsAlternative := makeLiteblocks(alternativeValSetChanges)
	testBisection.Witnesses[0] = MockProvider{}.New(liteBlocks[0].SignedHeader.Header.ChainID, liteBlocks)
	testBisection.ExpectedOutput = expectOutput
	return testBisection, statesPrimary, privValsPrimary, statesAlternative, privValsAlternative
}

func generateGeneralBisectionCase(
	description string,
	valSetChanges ValSetChanges,
	expectedBisections int32,
) (TestBisection, []st.State, types.PrivValidatorsByAddress) {

	liteBlocks, states, privVals := makeLiteblocks(valSetChanges)
	primary := MockProvider{}.New(liteBlocks[0].SignedHeader.Header.ChainID, liteBlocks)

	var witnesses []provider.Provider
	witnesses = append([]provider.Provider{}, primary)

	trustOptions := TrustOptions{}.make(liteBlocks[0].SignedHeader, TRUSTING_PERIOD, lite.DefaultTrustLevel)
	heightToVerify := int64(len(valSetChanges))

	testBisection := TestBisection{}.make(
		description,
		trustOptions,
		primary,
		witnesses,
		heightToVerify,
		now,
		expectedOutputNoError,
		expectedBisections,
	)

	return testBisection, states, privVals
}

// generateFirstBlock creates the first block of the chain
// with the given list of validators and timestamp
// Thus, It also calls the NewState() to initialize the state
// Returns the signedHeader and state after the first block is created
func generateFirstBlock(
	vals []*types.Validator,
	privVals types.PrivValidatorsByAddress,
	now time.Time,
) (types.SignedHeader, st.State, types.PrivValidatorsByAddress) {

	valSet := types.NewValidatorSet(vals)
	state := NewState("test-chain-01", valSet, valSet)

	return makeBlock(state, privVals, nil, now)
}

//TODO: Comment!
func generateFirstBlockWithNextValsUpdate(
	vals []*types.Validator,
	privVals types.PrivValidatorsByAddress,
	nextVals []*types.Validator,
	nextPrivVals types.PrivValidatorsByAddress,
	now time.Time,
) (types.SignedHeader, st.State, types.PrivValidatorsByAddress) {

	valSet := types.NewValidatorSet(vals)
	nextValSet := types.NewValidatorSet(nextVals)
	state := NewState("test-chain-01", valSet, nextValSet)

	return makeBlock(state, privVals, nextPrivVals, now)
}

func makeBlock(
	state st.State,
	privVals types.PrivValidatorsByAddress,
	nextPrivVals types.PrivValidatorsByAddress,
	now time.Time,
) (types.SignedHeader, st.State, types.PrivValidatorsByAddress) {
	txs := generateTxs()
	evidences := generateEvidences()
	lbh := state.LastBlockHeight + 1
	proposer := state.Validators.Proposer.Address

	// first block has a nil last commit
	block, partSet := state.MakeBlock(lbh, txs, nil, evidences, proposer)

	commit := generateCommit(block.Header, partSet, state.Validators, privVals, state.ChainID, now)

	state, privVals = updateState(state, commit.BlockID, privVals, nextPrivVals)

	signedHeader := types.SignedHeader{
		Header: &block.Header,
		Commit: commit,
	}

	return signedHeader, state, privVals
}

// Builds the Initial struct with given parameters
func generateInitial(signedHeader types.SignedHeader, nextValidatorSet types.ValidatorSet, trustingPeriod time.Duration, now time.Time) Initial {

	return Initial{
		SignedHeader:     signedHeader,
		NextValidatorSet: nextValidatorSet,
		TrustingPeriod:   trustingPeriod,
		Now:              now,
	}
}

// This one generates a "next" block,
// i.e. given the first block, this function can be used to build up successive blocks
func generateNextBlock(state st.State, privVals types.PrivValidatorsByAddress, lastCommit *types.Commit, now time.Time) (LiteBlock, st.State, types.PrivValidatorsByAddress) {

	txs := generateTxs()
	evidences := generateEvidences()
	lbh := state.LastBlockHeight + 1
	proposer := state.Validators.Proposer.Address

	block, partSet := state.MakeBlock(lbh, txs, lastCommit, evidences, proposer)

	commit := generateCommit(block.Header, partSet, state.Validators, privVals, state.ChainID, now)
	liteBlock := LiteBlock{
		SignedHeader: types.SignedHeader{
			Header: &block.Header,
			Commit: commit,
		},
		ValidatorSet:     *state.Validators.Copy(),     // dereferencing pointer
		NextValidatorSet: *state.NextValidators.Copy(), // dereferencing pointer
	}

	state, _ = updateState(state, commit.BlockID, privVals, nil)
	return liteBlock, state, privVals

}

// Similar to generateNextBlock
// It also takes in new validators and privVals to be added to the NextValidatorSet
// Calls the UpdateWithChangeSet function on state.NextValidatorSet for the same
// Also, you can specify the number of vals to be deleted from it
func generateNextBlockWithNextValsUpdate(
	state st.State,
	privVals types.PrivValidatorsByAddress,
	lastCommit *types.Commit,
	newVals []*types.Validator,
	newPrivVals types.PrivValidatorsByAddress,
	now time.Time,
) (LiteBlock, st.State, types.PrivValidatorsByAddress) {

	state.NextValidators = types.NewValidatorSet(newVals)

	// state.NextValidators.IncrementProposerPriority(1)

	txs := generateTxs()
	evidences := generateEvidences()
	lbh := state.LastBlockHeight + 1
	proposer := state.Validators.Proposer.Address

	block, partSet := state.MakeBlock(lbh, txs, lastCommit, evidences, proposer)
	commit := generateCommit(block.Header, partSet, state.Validators, privVals, state.ChainID, now)

	liteBlock := LiteBlock{
		SignedHeader: types.SignedHeader{
			Header: &block.Header,
			Commit: commit,
		},
		ValidatorSet:     *state.Validators.Copy(),     // dereferencing pointer
		NextValidatorSet: *state.NextValidators.Copy(), // dereferencing pointer
	}
	state, newPrivVals = updateState(state, commit.BlockID, privVals, newPrivVals)

	return liteBlock, state, newPrivVals
}

// Builds a general case containing initial and one lite block in input
// TODO: change name to genInitialAndInput
func generateGeneralCase(
	vals []*types.Validator,
	privVals types.PrivValidatorsByAddress,
) (Initial, []LiteBlock, st.State, types.PrivValidatorsByAddress) {

	var input []LiteBlock

	signedHeader, state, privVals := generateFirstBlock(vals, privVals, firstBlockTime)
	initial := generateInitial(signedHeader, *state.NextValidators, TRUSTING_PERIOD, now)
	liteBlock, state, _ := generateNextBlock(state, privVals, signedHeader.Commit, secondBlockTime)
	input = append(input, liteBlock)

	return initial, input, state, privVals
}

func generateInitialAndInputSkipBlocks(
	vals []*types.Validator,
	privVals types.PrivValidatorsByAddress,
	numOfBlocksToSkip int,
) (Initial, []LiteBlock, st.State, types.PrivValidatorsByAddress) {
	var input []LiteBlock

	signedHeader, state, privVals := generateFirstBlock(
		vals,
		privVals,
		firstBlockTime,
	)
	initial := generateInitial(signedHeader, *state.NextValidators, TRUSTING_PERIOD, now)

	blockTime := secondBlockTime
	for i := 0; i <= numOfBlocksToSkip; i++ {
		liteBlock, s, _ := generateNextBlock(state, privVals, signedHeader.Commit, blockTime)
		blockTime = blockTime.Add(5 * time.Second)
		state = s

		if i == numOfBlocksToSkip {
			input = append(input, liteBlock)
		}
	}

	return initial, input, state, privVals
}

func generateAndMakeGeneralTestCase(description string, vals []*types.Validator, privVals types.PrivValidatorsByAddress, expectedOutput string) TestCase {

	initial, input, _, _ := generateGeneralCase(vals, privVals)
	return makeTestCase(description, initial, input, expectedOutput)
}

func generateAndMakeNextValsUpdateTestCase(
	description string,
	initialVals []*types.Validator,
	initialPrivVals types.PrivValidatorsByAddress,
	nextVals []*types.Validator,
	nextPrivVals types.PrivValidatorsByAddress,
	expectedOutput string,
) TestCase {

	initial, input, _, _ := generateNextValsUpdateCase(initialVals, initialPrivVals, nextVals, nextPrivVals)
	return makeTestCase(description, initial, input, expectedOutput)
}

// Builds a case where next validator set changes
// makes initial, and input with 2 lite blocks
func generateNextValsUpdateCase(
	initialVals []*types.Validator,
	initialPrivVals types.PrivValidatorsByAddress,
	nextVals []*types.Validator,
	nextPrivVals types.PrivValidatorsByAddress,
) (Initial, []LiteBlock, st.State, types.PrivValidatorsByAddress) {

	var input []LiteBlock

	signedHeader, state, privVals := generateFirstBlock(initialVals, initialPrivVals, firstBlockTime)
	initial := generateInitial(signedHeader, *state.NextValidators, TRUSTING_PERIOD, now)

	liteBlock, state, privVals := generateNextBlockWithNextValsUpdate(state, privVals, signedHeader.Commit, nextVals, nextPrivVals, secondBlockTime)
	input = append(input, liteBlock)
	liteBlock, state, _ = generateNextBlock(state, privVals, liteBlock.SignedHeader.Commit, thirdBlockTime)
	input = append(input, liteBlock)

	return initial, input, state, privVals
}

// UPDATE -> mutex on PartSet and functions take pointer to valSet - have to use a pointer
// generateCommit takes in header, partSet from Block that was created,
// validator set, privVals, chain ID and a timestamp to create
// and return a commit type
// To be called after MakeBlock()
func generateCommit(
	header types.Header,
	partSet *types.PartSet,
	valSet *types.ValidatorSet,
	privVals []types.PrivValidator,
	chainID string,
	now time.Time,
) *types.Commit {
	blockID := types.BlockID{
		Hash: header.Hash(),
		PartsHeader: types.PartSetHeader{
			Hash:  partSet.Hash(),
			Total: partSet.Total(),
		},
	}
	voteSet := types.NewVoteSet(chainID, header.Height, 1, types.SignedMsgType(byte(types.PrecommitType)), valSet)

	commit, err := types.MakeCommit(blockID, header.Height, 1, voteSet, privVals, now)
	if err != nil {
		fmt.Println(err)
	}

	return commit
}
