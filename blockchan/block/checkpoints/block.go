package checkpoints

import (
	"encoding/json"
	"math/big"
	"sort"
	"sync"

	"github.com/SmartMeshFoundation/Spectrum/common"
	"github.com/pkg/errors"

	"github.com/SmartMeshFoundation/SmartPlasma/blockchan/block"
	"github.com/SmartMeshFoundation/SmartPlasma/merkle"
)

// CheckpointBlock defines the methods for standard Checkpoints block.
type CheckpointBlock interface {
	block.Block
	AddCheckpoint(uid, number *big.Int) error
	NumberOfCheckpoints() int64
	GetNonce(uid *big.Int) *big.Int
}

// Block is checkpoint block object.
type Block struct {
	mtx     sync.Mutex
	uIDs    []string
	numbers map[string]common.Hash
	tree    *merkle.Tree

	built bool
}

// NewBlock creates new Checkpoints block in memory.
func NewBlock() CheckpointBlock {
	return &Block{
		mtx:     sync.Mutex{},
		numbers: make(map[string]common.Hash),
	}
}

// Hash returns block hash.
func (bl *Block) Hash() common.Hash {
	if !bl.built {
		return common.Hash{}
	}
	return bl.tree.Root()
}

// AddCheckpoint adds a checkpoints to the block.
func (bl *Block) AddCheckpoint(uid, number *big.Int) error {
	if bl.built {
		return block.ErrAlreadyBuilt
	}

	bl.mtx.Lock()
	defer bl.mtx.Unlock()

	if _, ok := bl.numbers[uid.String()]; ok {
		return errors.Errorf("checkpoint for uid %s already"+
			" exist in the block", uid.String())
	}

	bl.uIDs = append(bl.uIDs, uid.String())
	bl.numbers[uid.String()] = common.BigToHash(number)
	return nil
}

// NumberOfCheckpoints returns number of checkpoints in the block.
func (bl *Block) NumberOfCheckpoints() int64 {
	return int64(len(bl.numbers))
}

// Build finalizes the block.
func (bl *Block) Build() (common.Hash, error) {
	if bl.built {
		return common.Hash{}, block.ErrAlreadyBuilt
	}

	bl.mtx.Lock()
	defer bl.mtx.Unlock()

	if !sort.StringsAreSorted(bl.uIDs) {
		sort.Strings(bl.uIDs)
	}

	tree, err := merkle.NewTree(bl.numbers, merkle.Depth257)
	if err != nil {
		return common.Hash{}, errors.Wrap(err, "failed to build block")
	}

	bl.tree = tree
	bl.built = true
	return bl.tree.Root(), nil
}

// IsBuilt if it is true then a block is already built.
func (bl *Block) IsBuilt() bool {
	return bl.built
}

// Marshal encodes block object to raw json data.
func (bl *Block) Marshal() ([]byte, error) {
	raw, err := json.Marshal(bl.numbers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode checkpoints")
	}

	return raw, nil
}

// Unmarshal decodes raw json data to block object.
func (bl *Block) Unmarshal(raw []byte) error {
	var checkpoints map[string]common.Hash

	if len(raw) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, &checkpoints); err != nil {
		return errors.Wrap(err, "failed to decode"+
			" checkpoints")
	}

	for uidStr, checkpoint := range checkpoints {
		id, ok := new(big.Int).SetString(uidStr, 10)
		if !ok {
			continue
		}

		if err := bl.AddCheckpoint(id, checkpoint.Big()); err != nil {
			return errors.Wrap(
				err, "failed to add checkpoint in the block")
		}
	}
	return nil
}

// CreateProof creates merkle proof for particular uid.
func (bl *Block) CreateProof(uid *big.Int) []byte {
	if !bl.built {
		return nil
	}
	return merkle.CreateProof(uid, merkle.Depth257, bl.tree.GetStructure(),
		bl.tree.DefaultNodes)
}

// GetNonce returns nonce for a particular UID.
func (bl *Block) GetNonce(uid *big.Int) *big.Int {
	if !bl.built {
		return nil
	}

	bl.mtx.Lock()
	defer bl.mtx.Unlock()

	return bl.tree.GetStructure()[0][uid.String()].Big()
}
