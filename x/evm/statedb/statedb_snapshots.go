package statedb

import (
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

// stateDBSnapshot holds a cached multi store that after a call to a
// precompile. It commits all the EVM dirty journals before the call to precompile and
// all the state changes in the precompile call.
type stateDBSnapshot struct {
	multiStore   storetypes.CacheMultiStore      // cached multi store before the precompile call, using for revert
	journal      *journal                        // cached journal before the precompile call, using for revert
	stateObjects map[common.Address]*stateObject // cached state objects before the precompile call, using for revert
	events       sdk.Events                      // cosmos events emitted before the precompile call, using for revert
}

type stateDBSnapshots struct {
	snapshots []stateDBSnapshot
}

func (ss *stateDBSnapshots) Revert(s *StateDB, idx int) {
	for i := len(ss.snapshots) - 1; i >= idx; i-- {
		cms := ss.snapshots[i].multiStore
		em := sdk.NewEventManager()
		em.EmitEvents(ss.snapshots[i].events)
		s.cachedCtx = s.cachedCtx.WithMultiStore(cms).WithEventManager(em)
		s.journal = ss.snapshots[i].journal
		s.stateObjects = ss.snapshots[i].stateObjects
		// For i > idx, even though the multistore will directly use the version of one of previous snapshots,
		// which makes the state changes in this journal no longer meaningful,
		// but it is still necessary to revert all journals here to rollback logs and refunds.
		if i > idx {
			s.journal.Revert(s, 0)
		}
	}
	if idx < len(ss.snapshots) {
		ss.snapshots = ss.snapshots[:idx]
	}
}

func (ss *stateDBSnapshots) GetCachedContextForPrecompile(s *StateDB) (sdk.Context, error) {
	if !s.hasCache {
		s.cachedCtx, _ = s.ctx.CacheContext()
		s.hasCache = true
	}
	ss.snapshots = append(ss.snapshots, stateDBSnapshot{
		multiStore:   s.cachedCtx.MultiStore().(sdk.CacheMultiStore).Copy(), // make a deep copy of store of cached ctx
		events:       s.cachedCtx.EventManager().Events(),
		journal:      s.journal,
		stateObjects: s.stateObjects,
	})
	if err := s.commitToContext(s.cachedCtx); err != nil {
		return sdk.Context{}, err
	}
	s.stateObjects = make(map[common.Address]*stateObject)
	s.journal = newJournal()
	return s.cachedCtx, nil
}

func (ss *stateDBSnapshots) Commit(s *StateDB) {
	if len(ss.snapshots) == 0 {
		return
	}
	s.ctx.EventManager().EmitEvents(s.cachedCtx.EventManager().Events())
	s.cachedCtx.MultiStore().(sdk.CacheMultiStore).Write()
}
