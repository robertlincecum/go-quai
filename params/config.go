// Cojyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

import (
	"fmt"
	"math/big"

	"github.com/dominant-strategies/go-quai/common"
)

// Genesis hashes to enforce below configs on.
var (
	// Progpow GenesisHashes
	ProgpowColosseumGenesisHash = common.HexToHash("0x3d2c4cc684a7b2c7764031058b457cdf09b946a62c7f3d6628f64386e7692d24")
	ProgpowGardenGenesisHash    = common.HexToHash("0x8975f8760d317524559986d595058d4d66c05d62e0eda1d740cdee56a25788a5")
	ProgpowOrchardGenesisHash   = common.HexToHash("0x5627aca8194b46ed071f92746ecf975542b12ce406905d715b4be8f044749956")
	ProgpowLocalGenesisHash     = common.HexToHash("0x6fcf67c1acb14a90b7507fd1c96ed66b49a3e426aea564614c5d2a16acefac78")
	ProgpowGalenaGenesisHash    = common.HexToHash("0x70b5bb5bd17d5ff4957a2f203c66b619a8be884f2ff4b3a75ad40d4bd3224aeb")

	// Blake3GenesisHashes
	Blake3PowColosseumGenesisHash = common.HexToHash("0x5746089cbee3cde719c3d0599e31504793028c68e5df7acff956e917f72866f5")
	Blake3PowGardenGenesisHash    = common.HexToHash("0x3fb55aff423ca143fd714e256530be8a34e21bd85f8dc62587f9fb559b739f36")
	Blake3PowOrchardGenesisHash   = common.HexToHash("0x998e99b46e56b90d56a36c77f32d223ad1dc7052c3318f58f305443910f0c9e6")
	Blake3PowLocalGenesisHash     = common.HexToHash("0x6181f4781316452abf66aaafa44568a4e9f56bb10c276c9d7b3964efea4fbc56")
	Blake3PowGalenaGenesisHash    = common.HexToHash("0x9a6a442ca5fe302c4f6ab3cb5ed81d3cdbddd0bde28a55ad864ac84cda07fea9")
)

var (
	// ColosseumChainConfig is the chain parameters to run a node on the Colosseum network.
	ProgpowColosseumChainConfig = &ChainConfig{
		ChainID:     big.NewInt(9000),
		Progpow:     new(ProgpowConfig),
		GenesisHash: ProgpowColosseumGenesisHash,
	}

	Blake3PowColosseumChainConfig = &ChainConfig{
		ChainID:     big.NewInt(9000),
		Blake3Pow:   new(Blake3powConfig),
		GenesisHash: Blake3PowColosseumGenesisHash,
	}

	// GardenChainConfig contains the chain parameters to run a node on the Garden test network.
	ProgpowGardenChainConfig = &ChainConfig{
		ChainID:     big.NewInt(12000),
		Progpow:     new(ProgpowConfig),
		GenesisHash: ProgpowGardenGenesisHash,
	}

	Blake3PowGardenChainConfig = &ChainConfig{
		ChainID:     big.NewInt(12000),
		Blake3Pow:   new(Blake3powConfig),
		GenesisHash: Blake3PowGardenGenesisHash,
	}

	// OrchardChainConfig contains the chain parameters to run a node on the Orchard test network.
	ProgpowOrchardChainConfig = &ChainConfig{
		ChainID:     big.NewInt(15000),
		Progpow:     new(ProgpowConfig),
		GenesisHash: ProgpowOrchardGenesisHash,
	}

	Blake3PowOrchardChainConfig = &ChainConfig{
		ChainID:     big.NewInt(15000),
		Blake3Pow:   new(Blake3powConfig),
		GenesisHash: Blake3PowOrchardGenesisHash,
	}

	// GalenaChainConfig contains the chain parameters to run a node on the Galena test network.
	ProgpowGalenaChainConfig = &ChainConfig{
		ChainID:     big.NewInt(17000),
		Blake3Pow:   new(Blake3powConfig),
		Progpow:     new(ProgpowConfig),
		GenesisHash: ProgpowGalenaGenesisHash,
	}

	Blake3PowGalenaChainConfig = &ChainConfig{
		ChainID:     big.NewInt(17000),
		Blake3Pow:   new(Blake3powConfig),
		GenesisHash: Blake3PowGalenaGenesisHash,
	}

	// LocalChainConfig contains the chain parameters to run a node on the Local test network.
	ProgpowLocalChainConfig = &ChainConfig{
		ChainID:     big.NewInt(1337),
		Progpow:     new(ProgpowConfig),
		GenesisHash: ProgpowLocalGenesisHash,
	}

	Blake3PowLocalChainConfig = &ChainConfig{
		ChainID:     big.NewInt(1337),
		Blake3Pow:   new(Blake3powConfig),
		GenesisHash: Blake3PowLocalGenesisHash,
	}

	// AllProgpowProtocolChanges contains every protocol change introduced
	// and accepted by the Quai core developers into the Progpow consensus.
	//
	// This configuration is intentionally not using keyed fields to force anyone
	// adding flags to the config to also have to set these fields.
	AllProgpowProtocolChanges = &ChainConfig{big.NewInt(1337), "progpow", new(Blake3powConfig), new(ProgpowConfig), common.Hash{}}

	TestChainConfig = &ChainConfig{big.NewInt(1), "progpow", new(Blake3powConfig), new(ProgpowConfig), common.Hash{}}
	TestRules       = TestChainConfig.Rules(new(big.Int))
)

// ChainConfig is the core config which determines the blockchain settings.
//
// ChainConfig is stored in the database on a per block basis. This means
// that any network, identified by its genesis block, can have its own
// set of configuration options.
type ChainConfig struct {
	ChainID *big.Int `json:"chainId"` // chainId identifies the current chain and is used for replay protection
	// Various consensus engines
	ConsensusEngine string
	Blake3Pow       *Blake3powConfig `json:"blake3pow,omitempty"`
	Progpow         *ProgpowConfig   `json:"progpow,omitempty"`
	GenesisHash     common.Hash
}

// Blake3powConfig is the consensus engine configs for proof-of-work based sealing.
type Blake3powConfig struct{}

// String implements the stringer interface, returning the consensus engine details.
func (c *Blake3powConfig) String() string {
	return "blake3pow"
}

// ProgpowConfig is the consensus engine configs for proof-of-work based sealing.
type ProgpowConfig struct{}

// String implements the stringer interface, returning the consensus engine details.
func (c *ProgpowConfig) String() string {
	return "progpow"
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	var engine interface{}
	switch {
	case c.Blake3Pow != nil:
		engine = c.Blake3Pow
	case c.Progpow != nil:
		engine = c.Progpow
	default:
		engine = "unknown"
	}
	return fmt.Sprintf("{ChainID: %v, Engine: %v}",
		c.ChainID,
		engine,
	)
}

func configNumEqual(x, y *big.Int) bool {
	if x == nil {
		return y == nil
	}
	if y == nil {
		return x == nil
	}
	return x.Cmp(y) == 0
}

// ConfigCompatError is raised if the locally-stored blockchain is initialised with a
// ChainConfig that would alter the past.
type ConfigCompatError struct {
	What string
	// block numbers of the stored and new configurations
	StoredConfig, NewConfig *big.Int
	// the block number to which the local chain must be rewound to correct the error
	RewindTo uint64
}

func newCompatError(what string, storedblock, newblock *big.Int) *ConfigCompatError {
	var rew *big.Int
	switch {
	case storedblock == nil:
		rew = newblock
	case newblock == nil || storedblock.Cmp(newblock) < 0:
		rew = storedblock
	default:
		rew = newblock
	}
	err := &ConfigCompatError{what, storedblock, newblock, 0}
	if rew != nil && rew.Sign() > 0 {
		err.RewindTo = rew.Uint64() - 1
	}
	return err
}

func (err *ConfigCompatError) Error() string {
	return fmt.Sprintf("mismatching %s in database (have %d, want %d, rewindto %d)", err.What, err.StoredConfig, err.NewConfig, err.RewindTo)
}

// Rules wraps ChainConfig and is merely syntactic sugar or can be used for functions
// that do not have or require information about the block.
//
// Rules is a one time interface meaning that it shouldn't be used in between transition
// phases.
type Rules struct {
	ChainID *big.Int
}

// Rules ensures c's ChainID is not nil.
func (c *ChainConfig) Rules(num *big.Int) Rules {
	chainID := c.ChainID
	if chainID == nil {
		chainID = new(big.Int)
	}
	return Rules{
		ChainID: new(big.Int).Set(chainID),
	}
}
