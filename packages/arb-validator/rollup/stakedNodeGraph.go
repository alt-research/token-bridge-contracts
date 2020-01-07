/*
* Copyright 2019-2020, Offchain Labs, Inc.
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*    http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package rollup

import (
	"bytes"
	"sort"

	"github.com/offchainlabs/arbitrum/packages/arb-util/utils"

	"github.com/offchainlabs/arbitrum/packages/arb-validator/ethbridge"

	"github.com/offchainlabs/arbitrum/packages/arb-validator/structures"

	"github.com/ethereum/go-ethereum/common"
	"github.com/offchainlabs/arbitrum/packages/arb-util/machine"
)

//go:generate bash -c "protoc -I$(go list -f '{{ .Dir }}' -m github.com/offchainlabs/arbitrum/packages/arb-util) -I. --go_out=paths=source_relative:. *.proto"

var zeroBytes32 [32]byte // deliberately zeroed

type StakedNodeGraph struct {
	*NodeGraph
	stakers *StakerSet
}

func NewStakedNodeGraph(machine machine.Machine, params structures.ChainParams) *StakedNodeGraph {
	return &StakedNodeGraph{
		NodeGraph: NewNodeGraph(machine, params),
		stakers:   NewStakerSet(),
	}
}

func (chain *StakedNodeGraph) MarshalForCheckpoint(ctx structures.CheckpointContext) *StakedNodeGraphBuf {
	var allStakers []*StakerBuf
	chain.stakers.forall(func(staker *Staker) {
		allStakers = append(allStakers, staker.MarshalToBuf())
	})
	return &StakedNodeGraphBuf{
		NodeGraph: chain.NodeGraph.MarshalForCheckpoint(ctx),
		Stakers:   allStakers,
	}
}

func (m *StakedNodeGraphBuf) UnmarshalFromCheckpoint(ctx structures.RestoreContext) *StakedNodeGraph {
	chain := &StakedNodeGraph{
		NodeGraph: m.NodeGraph.UnmarshalFromCheckpoint(ctx),
		stakers:   NewStakerSet(),
	}
	for _, stakerBuf := range m.Stakers {
		chain.stakers.Add(stakerBuf.Unmarshal(chain))
	}
	return chain
}

func (s *StakedNodeGraph) Equals(s2 *StakedNodeGraph) bool {
	return s.NodeGraph.Equals(s2.NodeGraph) &&
		s.stakers.Equals(s2.stakers)
}

func (chain *StakedNodeGraph) CreateStake(ev ethbridge.StakeCreatedEvent, currentTime structures.TimeTicks) {
	chain.stakers.Add(&Staker{
		ev.Staker,
		chain.nodeFromHash[ev.NodeHash],
		currentTime,
		common.Address{},
	})
}

func (chain *StakedNodeGraph) MoveStake(stakerAddr common.Address, nodeHash [32]byte) {
	staker := chain.stakers.Get(stakerAddr)
	staker.location.numStakers--
	// no need to consider pruning staker.location, because a successor of it is getting a stake
	staker.location = chain.nodeFromHash[nodeHash]
	staker.location.numStakers++
}

func (chain *StakedNodeGraph) RemoveStake(stakerAddr common.Address) {
	staker := chain.stakers.Get(stakerAddr)
	staker.location.numStakers--
	chain.considerPruningNode(staker.location)
	chain.stakers.Delete(staker)
}

func (chain *StakedNodeGraph) NewChallenge(contract, asserter, challenger common.Address, kind structures.ChildType) {
	chain.stakers.Get(asserter).challenge = contract
	chain.stakers.Get(challenger).challenge = contract
}

func (chain *StakedNodeGraph) ChallengeResolved(contract, winner, loser common.Address) {
	chain.stakers.Get(winner).challenge = common.Address{}
	chain.RemoveStake(loser)
}

type SortableAddressList []common.Address

func (sa SortableAddressList) Len() int {
	return len(sa)
}

func (sa SortableAddressList) Less(i, j int) bool {
	return bytes.Compare(sa[i][:], sa[j][:]) < 0
}

func (sa SortableAddressList) Swap(i, j int) {
	sa[i], sa[j] = sa[j], sa[i]
}

func (sng *StakedNodeGraph) generateAlignedStakersProof(
	confirmingNode *Node,
	deadline structures.TimeTicks,
) (stakerAddrs []common.Address, proof [][32]byte, offsets []uint64) {
	sng.stakers.forall(func(st *Staker) {
		stakerAddrs = append(stakerAddrs, st.address)
	})
	sort.Sort(SortableAddressList(stakerAddrs))

	for _, sa := range stakerAddrs {
		staker := sng.stakers.Get(sa)
		if staker.creationTime.Cmp(deadline) >= 0 {
			offsets = append(offsets, uint64(len(proof)))
			subProof := GeneratePathProof(confirmingNode, staker.location)
			proof = append(proof, subProof...)
		}
	}
	return
}

func (chain *StakedNodeGraph) generateStakerPruneInfo() ([]recoverStakeMootedParams, []recoverStakeOldParams) {
	mootedToDo := []recoverStakeMootedParams{}
	oldToDo := []recoverStakeOldParams{}
	chain.stakers.forall(func(staker *Staker) {
		stakerAncestor, _, _, err := chain.GetConflictAncestor(staker.location, chain.latestConfirmed)
		if err == nil {
			mootedToDo = append(mootedToDo, recoverStakeMootedParams{
				addr:     staker.address,
				ancestor: stakerAncestor.prev,
				lcProof:  GeneratePathProof(stakerAncestor.prev, chain.latestConfirmed),
				stProof:  GeneratePathProof(stakerAncestor.prev, staker.location),
			})
		} else if staker.location.depth < chain.latestConfirmed.depth {
			oldToDo = append(oldToDo, recoverStakeOldParams{
				addr:  staker.address,
				proof: GeneratePathProof(staker.location, chain.latestConfirmed),
			})
		}
	})
	return mootedToDo, oldToDo
}

type challengeOpportunity struct {
	asserter              common.Address
	challenger            common.Address
	prevNodeHash          [32]byte
	deadlineTicks         structures.TimeTicks
	asserterNodeType      structures.ChildType
	challengerNodeType    structures.ChildType
	asserterVMProtoHash   [32]byte
	challengerVMProtoHash [32]byte
	asserterProof         [][32]byte
	challengerProof       [][32]byte
	asserterDataHash      [32]byte
	asserterPeriodTicks   structures.TimeTicks
	challengerNodeHash    [32]byte
}

func (chain *StakedNodeGraph) checkChallengeOpportunityPair(staker1, staker2 *Staker) *challengeOpportunity {
	if !utils.AddressIsZero(staker1.challenge) || !utils.AddressIsZero(staker2.challenge) {
		return nil
	}
	staker1Ancestor, staker2Ancestor, err := GetConflictAncestor(staker1.location, staker2.location)
	if err != nil {
		return nil
	}
	linkType1 := staker1Ancestor.linkType
	linkType2 := staker2Ancestor.linkType

	var asserterStaker *Staker
	var asserterAncestor *Node
	var challengerStaker *Staker
	var challengerAncestor *Node
	if linkType1 < linkType2 {
		asserterStaker = staker2
		asserterAncestor = staker2Ancestor
		challengerStaker = staker1
		challengerAncestor = staker1Ancestor
	} else {
		asserterStaker = staker1
		asserterAncestor = staker1Ancestor
		challengerStaker = staker2
		challengerAncestor = staker2Ancestor
	}

	asserterDataHash, asserterPeriodTicks := asserterAncestor.ChallengeNodeData(chain.params)

	return &challengeOpportunity{
		asserter:              asserterStaker.address,
		challenger:            challengerStaker.address,
		prevNodeHash:          asserterAncestor.prev.hash,
		deadlineTicks:         asserterAncestor.deadline,
		asserterNodeType:      asserterAncestor.linkType,
		challengerNodeType:    challengerAncestor.linkType,
		asserterVMProtoHash:   asserterAncestor.vmProtoData.Hash(),
		challengerVMProtoHash: challengerAncestor.vmProtoData.Hash(),
		asserterProof:         GeneratePathProof(asserterAncestor, asserterStaker.location),
		challengerProof:       GeneratePathProof(challengerAncestor, challengerStaker.location),
		asserterDataHash:      asserterDataHash,
		asserterPeriodTicks:   asserterPeriodTicks,
		challengerNodeHash:    challengerAncestor.nodeDataHash,
	}
}

func (chain *StakedNodeGraph) checkChallengeOpportunityAny(staker *Staker) *challengeOpportunity {
	if !utils.AddressIsZero(staker.challenge) {
		return nil
	}
	var ret *challengeOpportunity
	chain.stakers.forall(func(staker2 *Staker) {
		if !staker2.Equals(staker) {
			opp := chain.checkChallengeOpportunityPair(staker, staker2)
			if opp != nil {
				ret = opp
				return
			}
		}
	})
	return ret
}

func (chain *StakedNodeGraph) checkChallengeOpportunityAllPairs() []*challengeOpportunity {
	ret := []*challengeOpportunity{}
	stakers := []*Staker{}
	chain.stakers.forall(func(s *Staker) {
		stakers = append(stakers, s)
	})
	for i, s1 := range stakers {
		for j := i + 1; j < len(stakers); j++ {
			opp := chain.checkChallengeOpportunityPair(s1, stakers[j])
			if opp != nil {
				ret = append(ret, opp)
				break
			}
		}
	}
	return ret
}
