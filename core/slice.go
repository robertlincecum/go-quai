package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/spruce-solutions/go-quai/common"
	"github.com/spruce-solutions/go-quai/consensus"
	"github.com/spruce-solutions/go-quai/core/rawdb"
	"github.com/spruce-solutions/go-quai/core/types"
	"github.com/spruce-solutions/go-quai/core/vm"
	"github.com/spruce-solutions/go-quai/ethclient/quaiclient"
	"github.com/spruce-solutions/go-quai/ethdb"
	"github.com/spruce-solutions/go-quai/log"
	"github.com/spruce-solutions/go-quai/params"
)

type Slice struct {
	hc *HeaderChain

	config *params.ChainConfig
	engine consensus.Engine

	quit chan struct{} // slice quit channel

	domClient    *quaiclient.Client   // domClient is used to check if a given dominant block in the chain is canonical in dominant chain.
	subClients   []*quaiclient.Client // subClinets is used to check is a coincident block is valid in the subordinate context
	futureBlocks *lru.Cache

	wg sync.WaitGroup // slice processing wait group for shutting down
}

func NewSlice(db ethdb.Database, cacheConfig *CacheConfig, chainConfig *params.ChainConfig, domClientUrl string, subClientUrls []string, engine consensus.Engine, vmConfig vm.Config) (*Slice, error) {

	sl := &Slice{
		config: chainConfig,
		engine: engine,
	}

	futureBlocks, _ := lru.New(maxFutureBlocks)
	sl.futureBlocks = futureBlocks

	var err error
	sl.hc, err = NewHeaderChain(db, cacheConfig, chainConfig, vmConfig)
	if err != nil {
		return nil, err
	}

	// only set the domClient if the chain is not prime
	if types.QuaiNetworkContext != params.PRIME {
		sl.domClient = MakeDomClient(domClientUrl)
	}

	sl.subClients = make([]*quaiclient.Client, 3)
	// only set the subClients if the chain is not region
	if types.QuaiNetworkContext != params.ZONE {
		go func() {
			sl.subClients = MakeSubClients(subClientUrls)
		}()
	}

	return sl, nil
}

// MakeDomClient creates the quaiclient for the given domurl
func MakeDomClient(domurl string) *quaiclient.Client {
	if domurl == "" {
		log.Crit("dom client url is empty")
	}
	domClient, err := quaiclient.Dial(domurl)
	if err != nil {
		log.Crit("Error connecting to the dominant go-quai client", "err", err)
	}
	return domClient
}

// MakeSubClients creates the quaiclient for the given suburls
func MakeSubClients(suburls []string) []*quaiclient.Client {
	subClients := make([]*quaiclient.Client, 3)
	for i, suburl := range suburls {
		if suburl == "" {
			log.Warn("sub client url is empty")
		}
		subClient, err := quaiclient.Dial(suburl)
		if err != nil {
			log.Crit("Error connecting to the subordinate go-quai client for index", "index", i, " err ", err)
		}
		subClients[i] = subClient
	}
	return subClients
}

// Config retrieves the slice's chain configuration.
func (sl *Slice) Config() *params.ChainConfig { return sl.config }

// Engine retrieves the header chain's consensus engine.
func (sl *Slice) Engine() consensus.Engine { return sl.engine }

// Append
func (sl *Slice) Append(block *types.Block) error {
	order, err := sl.GetDifficultyOrder(block.Header())
	if err != nil {
		return err
	}

	_, err = sl.PCRC(block.Header(), order)
	if err != nil {
		return err
	}

	err = sl.hc.Append(block)
	if err != nil {
		return err
	}

	td, err := sl.CalcTd(block.Header())
	if err != nil {
		return err
	}

	// write the tds
	rawdb.WriteTd(sl.hc.headerDb, block.Hash(), block.NumberU64(), td)

	// We have a new possible head call HLCR to potentially set
	reorg := sl.HLCR(sl.hc.GetTd(sl.hc.currentHeaderHash, sl.hc.CurrentHeader().Number64()), td)

	if reorg {
		err = sl.hc.SetCurrentHeader(block.Header())
		if err != nil {
			return err
		}
	}

	return nil
}

// The purpose of the Previous Coincident Reference Check (PCRC) is to establish
// that we have linked untwisted chains prior to checking HLCR & applying external state transfers.
// NOTE: note that it only guarantees linked & untwisted back to the prime terminus, assuming the
// prime termini match. To check deeper than that, you need to iteratively apply PCRC to get that guarantee.
func (sl *Slice) PCRC(header *types.Header, headerOrder int) (types.PCRCTermini, error) {

	if header.Number[types.QuaiNetworkContext].Cmp(big.NewInt(0)) == 0 {
		return types.PCRCTermini{}, nil
	}

	slice := header.Location

	switch types.QuaiNetworkContext {
	case params.PRIME:
		PTP, err := sl.PreviousValidCoincidentOnPath(header, slice, params.PRIME, params.PRIME, true)
		if err != nil {
			return types.PCRCTermini{}, err
		}
		PRTP, err := sl.PreviousValidCoincidentOnPath(header, slice, params.PRIME, params.PRIME, false)
		if err != nil {
			return types.PCRCTermini{}, err
		}

		if sl.subClients[slice[0]-1] == nil {
			return types.PCRCTermini{}, nil
		}
		PCRCTermini, err := sl.subClients[slice[0]-1].CheckPCRC(context.Background(), header, headerOrder)
		if err != nil {
			return types.PCRCTermini{}, err
		}

		if (PCRCTermini.PTR == common.Hash{} || PCRCTermini.PRTR == common.Hash{}) {
			return PCRCTermini, consensus.ErrSliceNotSynced
		}

		PCRCTermini.PTP = PTP.Hash()
		PCRCTermini.PRTP = PRTP.Hash()

		if (PTP.Hash() != PCRCTermini.PTR) && (PCRCTermini.PTR != PCRCTermini.PTZ) && (PCRCTermini.PTZ != PTP.Hash()) {
			return types.PCRCTermini{}, errors.New("there exists a Prime twist (PTP != PTR != PTZ")
		}
		if PRTP.Hash() != PCRCTermini.PRTR {
			return types.PCRCTermini{}, errors.New("there exists a Prime twist (PRTP != PRTR")
		}

		return PCRCTermini, nil

	case params.REGION:
		RTR, err := sl.PreviousValidCoincidentOnPath(header, slice, params.REGION, params.REGION, true)
		if err != nil {
			return types.PCRCTermini{}, err
		}

		if sl.subClients[slice[1]-1] == nil {
			return types.PCRCTermini{}, nil
		}

		PCRCTermini, err := sl.subClients[slice[1]-1].CheckPCRC(context.Background(), header, headerOrder)
		if err != nil {
			return types.PCRCTermini{}, err
		}

		if (PCRCTermini.RTZ == common.Hash{}) {
			return PCRCTermini, consensus.ErrSliceNotSynced
		}

		if RTR.Hash() != PCRCTermini.RTZ {
			return types.PCRCTermini{}, errors.New("there exists a Region twist (RTR != RTZ)")
		}
		if headerOrder < params.REGION {
			PTR, err := sl.PreviousValidCoincidentOnPath(header, slice, params.PRIME, params.REGION, true)
			if err != nil {
				return types.PCRCTermini{}, err
			}
			PRTR, err := sl.PreviousValidCoincidentOnPath(header, slice, params.PRIME, params.REGION, false)
			if err != nil {
				return types.PCRCTermini{}, err
			}

			PCRCTermini.PTR = PTR.Hash()
			PCRCTermini.PRTR = PRTR.Hash()
		}
		return PCRCTermini, nil

	case params.ZONE:
		PCRCTermini := types.PCRCTermini{}

		// only compute PTZ and RTZ on the coincident block in zone.
		// PTZ and RTZ are essentially a signaling mechanism to know that we are building on the right terminal header.
		// So running this only on a coincident block makes sure that the zones can move and sync past the coincident.
		// Just run RTZ to make sure that its linked. This check decouples this signaling and linking paradigm.
		if headerOrder < params.REGION {
			PTZ, err := sl.PreviousValidCoincidentOnPath(header, slice, params.PRIME, params.ZONE, true)
			if err != nil {
				return types.PCRCTermini{}, err
			}
			PCRCTermini.PTZ = PTZ.Hash()
		}

		if headerOrder < params.ZONE {
			RTZ, err := sl.PreviousValidCoincidentOnPath(header, slice, params.REGION, params.ZONE, true)
			if err != nil {
				return types.PCRCTermini{}, err
			}
			PCRCTermini.RTZ = RTZ.Hash()
		}

		return PCRCTermini, nil
	}
	return types.PCRCTermini{}, errors.New("running in unsupported context")
}

// PreviousValidCoincidentOnPath searches the path for a cononical block of specified order in the specified slice
//     *slice - The zone location which defines the slice in which we are validating
//     *order - The order of the conincidence that is desired
//     *path - Search among ancestors of this path in the specified slice
func (sl *Slice) PreviousValidCoincidentOnPath(header *types.Header, slice []byte, order, path int, fullSliceEqual bool) (*types.Header, error) {
	prevTerminalHeader := header
	for {
		if prevTerminalHeader.Number[types.QuaiNetworkContext].Cmp(big.NewInt(0)) == 0 {
			return sl.hc.GetHeaderByHash(sl.Config().GenesisHashes[0]), nil
		}

		terminalHeader, err := sl.PreviousCoincidentOnPath(prevTerminalHeader, slice, order, fullSliceEqual)
		if err != nil {
			return nil, err
		}

		fmt.Println("Running PVCOP for header: ", header.Hash(), header.Number, "terminal Header", terminalHeader.Hash(), terminalHeader.Number)

		if terminalHeader.Number[types.QuaiNetworkContext].Cmp(big.NewInt(0)) == 0 {
			return sl.hc.GetHeaderByHash(sl.Config().GenesisHashes[0]), nil
		}

		// If the current header is dominant coincident check the status with the dom node
		if order < types.QuaiNetworkContext {
			status := sl.hc.GetHeaderByHash(terminalHeader.Hash())
			fmt.Println("terminal Header status", status)
			if status != nil {
				if prevTerminalHeader.Hash() != header.Hash() {
					return nil, errors.New("subordinate terminus mismatch")
				}
				return terminalHeader, nil
			}
		} else if order == types.QuaiNetworkContext {
			return terminalHeader, err
		}

		prevTerminalHeader = terminalHeader
	}
}

// PreviousCoincidentOnPath searches the path for a block of specified order in the specified slice
//     *slice - The zone location which defines the slice in which we are validating
//     *order - The order of the conincidence that is desired
//     *path - Search among ancestors of this path in the specified slice
func (sl *Slice) PreviousCoincidentOnPath(header *types.Header, slice []byte, order int, fullSliceEqual bool) (*types.Header, error) {

	if header.Number[types.QuaiNetworkContext].Cmp(big.NewInt(0)) == 0 {
		return sl.hc.GetHeaderByHash(sl.hc.Config().GenesisHashes[0]), nil
	}

	if err := sl.hc.CheckContext(order); err != nil {
		return nil, err
	}
	if err := sl.hc.CheckLocationRange(slice); err != nil {
		return nil, err
	}

	for {
		// If block header is Genesis return it as coincident
		if header.Number[types.QuaiNetworkContext].Cmp(big.NewInt(1)) == 0 {
			return sl.hc.GetHeaderByHash(sl.hc.Config().GenesisHashes[0]), nil
		}

		// Get previous header on local chain by hash
		prevHeader := sl.hc.GetHeaderByHash(header.ParentHash[types.QuaiNetworkContext])
		if prevHeader == nil {
			return nil, consensus.ErrSliceNotSynced
		}
		// Increment previous header
		header = prevHeader

		// Find the order of the header
		difficultyOrder, err := sl.GetDifficultyOrder(header)
		if err != nil {
			return nil, err
		}

		// If we have reached a coincident block of desired order in our desired slice
		var equal bool
		if fullSliceEqual {
			equal = bytes.Equal(header.Location, slice)
		} else {
			equal = header.Location[0] == slice[0]
		}
		if equal && difficultyOrder <= order {
			return header, nil
		}
	}
}

// HLCR does hierarchical comparison of two difficulty tuples and returns true if second tuple is greater than the first
func (sl *Slice) HLCR(localDifficulties []*big.Int, externDifficulties []*big.Int) bool {

	if localDifficulties[types.QuaiNetworkContext].Cmp(externDifficulties[types.QuaiNetworkContext]) < 0 {
		return true
	} else if localDifficulties[types.QuaiNetworkContext].Cmp(externDifficulties[types.QuaiNetworkContext]) > 0 {
		return false
	}

	return false
}

func (sl *Slice) procFutureBlocks() {
	blocks := make([]*types.Block, 0, sl.futureBlocks.Len())
	for _, hash := range sl.futureBlocks.Keys() {
		if block, exist := sl.futureBlocks.Peek(hash); exist {
			blocks = append(blocks, block.(*types.Block))
		}
	}
	if len(blocks) > 0 {
		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].NumberU64() < blocks[j].NumberU64()
		})
		// Insert one by one as chain insertion needs contiguous ancestry between blocks
		for i := range blocks {
			sl.Append(blocks[i])
		}
	}
}

// addFutureBlock checks if the block is within the max allowed window to get
// accepted for future processing, and returns an error if the block is too far
// ahead and was not added.
func (sl *Slice) addFutureBlock(block *types.Block) error {
	max := uint64(time.Now().Unix() + maxTimeFutureBlocks)
	if block.Time() > max {
		return fmt.Errorf("future block timestamp %v > allowed %v", block.Time(), max)
	}
	if !sl.futureBlocks.Contains(block.Hash()) {
		sl.futureBlocks.Add(block.Hash(), block)
	}
	return nil
}

func (sl *Slice) update() {
	futureTimer := time.NewTicker(1 * time.Second)
	defer futureTimer.Stop()
	defer sl.wg.Done()
	for {
		select {
		case <-futureTimer.C:
			sl.procFutureBlocks()
		case <-sl.quit:
			return
		}
	}
}

// CalcTd calculates the TD of the given header using PCRC and CalcHLCRNetDifficulty.
func (sl *Slice) CalcTd(header *types.Header) ([]*big.Int, error) {
	// Iterate ancestors, stopping when a TD value is found in cache or a coincident block is found.
	// If coincident is found, ask dom client for TD at that block
	aggDiff := new(big.Int)
	cursor := header
	for {
		// First, check if this block's TD is already cached
		td := sl.hc.GetTd(cursor.Hash(), (*cursor.Number[types.QuaiNetworkContext]).Uint64())
		if td != nil {
			// Add the difficulty we accumulated up till this block
			blockTd := td[types.QuaiNetworkContext]
			td[types.QuaiNetworkContext] = blockTd.Add(blockTd, aggDiff)
			return td, nil
		}

		// If not cached, check if this block coincides with a dominant chain
		order, err := sl.GetDifficultyOrder(cursor)
		if err != nil {
			return nil, err
		} else if order < types.QuaiNetworkContext {
			// TODO: Ask dom to CalcTd on coincident block
			td, err = sl.domClient.CalcTd(context.Background(), header)
			err = errors.New("TODO: Ask dom to CalcTd")
			if err != nil {
				return nil, err
			} else {
				blockTd := td[types.QuaiNetworkContext]
				td[types.QuaiNetworkContext] = blockTd.Add(blockTd, aggDiff)
			}
		}

		// If not cached AND not coincident, aggregate the difficulty and iterate to the parent
		aggDiff = aggDiff.Add(aggDiff, cursor.Difficulty[types.QuaiNetworkContext])
		parentHash := cursor.ParentHash[types.QuaiNetworkContext]
		cursor = sl.hc.GetHeader(cursor.Hash(), (*cursor.Number[types.QuaiNetworkContext]).Uint64())
		if cursor == nil {
			return nil, fmt.Errorf("Unable to find parent: %s", parentHash)
		}
	}
}

// This function determines the difficulty order of a block
func (sl *Slice) GetDifficultyOrder(header *types.Header) (int, error) {
	//TODO: need to write this function
	return 0, nil
}
