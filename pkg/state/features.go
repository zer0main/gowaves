package state

import (
	"encoding/binary"
	"log"

	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/keyvalue"
	"github.com/wavesplatform/gowaves/pkg/settings"
)

const (
	activatedFeaturesRecordSize = 8 + crypto.SignatureSize
	approvedFeaturesRecordSize  = 8 + crypto.SignatureSize
	votesFeaturesRecordSize     = 8 + crypto.SignatureSize
)

type activatedFeaturesRecord struct {
	activationHeight uint64
	blockID          crypto.Signature
}

func (r *activatedFeaturesRecord) marshalBinary() ([]byte, error) {
	res := make([]byte, activatedFeaturesRecordSize)
	binary.BigEndian.PutUint64(res[:8], r.activationHeight)
	copy(res[8:], r.blockID[:])
	return res, nil
}

func (r *activatedFeaturesRecord) unmarshalBinary(data []byte) error {
	if len(data) != activatedFeaturesRecordSize {
		return errors.New("invalid data size")
	}
	r.activationHeight = binary.BigEndian.Uint64(data[:8])
	copy(r.blockID[:], data[8:])
	return nil
}

type approvedFeaturesRecord struct {
	approvalHeight uint64
	blockID        crypto.Signature
}

func (r *approvedFeaturesRecord) marshalBinary() ([]byte, error) {
	res := make([]byte, approvedFeaturesRecordSize)
	binary.BigEndian.PutUint64(res[:8], r.approvalHeight)
	copy(res[8:], r.blockID[:])
	return res, nil
}

func (r *approvedFeaturesRecord) unmarshalBinary(data []byte) error {
	if len(data) != approvedFeaturesRecordSize {
		return errors.New("invalid data size")
	}
	r.approvalHeight = binary.BigEndian.Uint64(data[:8])
	copy(r.blockID[:], data[8:])
	return nil
}

type votesFeaturesRecord struct {
	votesNum uint64
	blockID  crypto.Signature
}

func (r *votesFeaturesRecord) marshalBinary() ([]byte, error) {
	res := make([]byte, votesFeaturesRecordSize)
	binary.BigEndian.PutUint64(res[:8], r.votesNum)
	copy(res[8:], r.blockID[:])
	return res, nil
}

func (r *votesFeaturesRecord) unmarshalBinary(data []byte) error {
	if len(data) != votesFeaturesRecordSize {
		return errors.New("invalid data size")
	}
	r.votesNum = binary.BigEndian.Uint64(data[:8])
	copy(r.blockID[:], data[8:])
	return nil
}

type features struct {
	db                  keyvalue.IterableKeyVal
	dbBatch             keyvalue.Batch
	hs                  *historyStorage
	settings            *settings.BlockchainSettings
	definedFeaturesInfo map[settings.Feature]settings.FeatureInfo
}

func newFeatures(
	db keyvalue.IterableKeyVal,
	dbBatch keyvalue.Batch,
	hs *historyStorage,
	settings *settings.BlockchainSettings,
	definedFeaturesInfo map[settings.Feature]settings.FeatureInfo,
) (*features, error) {
	return &features{db, dbBatch, hs, settings, definedFeaturesInfo}, nil
}

// addVote adds vote for feature by its featureID at given blockID.
func (f *features) addVote(featureID int16, blockID crypto.Signature) error {
	key := votesFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return err
	}
	prevVotes, err := f.featureVotes(featureID)
	if err != nil {
		return err
	}
	record := &votesFeaturesRecord{prevVotes + 1, blockID}
	recordBytes, err := record.marshalBinary()
	if err != nil {
		return err
	}
	return f.hs.set(featureVote, keyBytes, recordBytes)
}

func (f *features) featureVotes(featureID int16) (uint64, error) {
	key := votesFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return 0, err
	}
	recordBytes, err := f.hs.getFresh(featureVote, keyBytes, true)
	if err == keyvalue.ErrNotFound || err == errEmptyHist {
		// 0 votes for unknown feature.
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var record votesFeaturesRecord
	if err := record.unmarshalBinary(recordBytes); err != nil {
		return 0, err
	}
	return record.votesNum, nil
}

func (f *features) printActivationLog(featureID int16) {
	info, ok := f.definedFeaturesInfo[settings.Feature(featureID)]
	if ok {
		log.Printf("Activating feature %d (%s).\n", featureID, info.Description)
	} else {
		log.Printf("Activating UNKNOWN feature %d.\n", featureID)
	}
	if !ok || !info.Implemented {
		log.Printf("FATAL: UNKNOWN/UNIMPLEMENTED feature has been activated on the blockchain!\n")
		log.Printf("FOR THIS REASON THE NODE IS STOPPED AUTOMATICALLY.\n")
		log.Fatalf("PLEASE, UPDATE THE NODE IMMEDIATELY!\n")
	}
}

func (f *features) activateFeature(featureID int16, r *activatedFeaturesRecord) error {
	key := activatedFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return err
	}
	recordBytes, err := r.marshalBinary()
	if err != nil {
		return err
	}
	f.printActivationLog(featureID)
	return f.hs.set(activatedFeature, keyBytes, recordBytes)
}

func (f *features) isActivated(featureID int16) (bool, error) {
	key := activatedFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return false, err
	}
	_, err = f.hs.get(activatedFeature, keyBytes, true)
	if err == keyvalue.ErrNotFound || err == errEmptyHist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (f *features) activationHeight(featureID int16) (uint64, error) {
	key := activatedFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return 0, err
	}
	recordBytes, err := f.hs.get(activatedFeature, keyBytes, true)
	if err != nil {
		return 0, err
	}
	var record activatedFeaturesRecord
	if err := record.unmarshalBinary(recordBytes); err != nil {
		return 0, err
	}
	return record.activationHeight, nil
}

func (f *features) printApprovalLog(featureID int16) {
	info, ok := f.definedFeaturesInfo[settings.Feature(featureID)]
	if ok {
		log.Printf("Approving feature %d (%s).\n", featureID, info.Description)
	} else {
		log.Printf("Approving UNKNOWN feature %d.\n", featureID)
	}
	if !ok || !info.Implemented {
		log.Printf("WARNING: UNKNOWN/UNIMPLEMENTED feature has been approved on the blockchain!\n")
		log.Printf("PLEASE UPDATE THE NODE AS SOON AS POSSILE!\n")
		log.Printf("OTHERWISE THE NODE WILL BE STOPPED OR FORKED UPON FEATURE ACTIVATION.\n")
	}
}

func (f *features) approveFeature(featureID int16, r *approvedFeaturesRecord) error {
	key := approvedFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return err
	}
	recordBytes, err := r.marshalBinary()
	if err != nil {
		return err
	}
	f.printApprovalLog(featureID)
	return f.hs.set(approvedFeature, keyBytes, recordBytes)
}

func (f *features) isApproved(featureID int16) (bool, error) {
	key := approvedFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return false, err
	}
	_, err = f.hs.get(approvedFeature, keyBytes, true)
	if err == keyvalue.ErrNotFound || err == errEmptyHist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (f *features) approvalHeight(featureID int16) (uint64, error) {
	key := approvedFeaturesKey{featureID: featureID}
	keyBytes, err := key.bytes()
	if err != nil {
		return 0, err
	}
	recordBytes, err := f.hs.get(approvedFeature, keyBytes, true)
	if err != nil {
		return 0, err
	}
	var record approvedFeaturesRecord
	if err := record.unmarshalBinary(recordBytes); err != nil {
		return 0, err
	}
	return record.approvalHeight, nil
}

func (f *features) isElected(height uint64, featureID int16) (bool, error) {
	votes, err := f.featureVotes(featureID)
	if err != nil {
		return false, err
	}
	return votes >= f.settings.VotesForFeatureElection(height), nil
}

// Check voting results, update approval list, reset voting list.
func (f *features) approveFeatures(curHeight uint64, curBlockID crypto.Signature) error {
	iter, err := f.db.NewKeyIterator([]byte{votesFeaturesKeyPrefix})
	if err != nil {
		return err
	}
	defer func() {
		iter.Release()
		if err := iter.Error(); err != nil {
			log.Fatalf("Iterator error: %v", err)
		}
	}()

	for iter.Next() {
		// Iterate the voting list.
		key := keyvalue.SafeKey(iter)
		var k votesFeaturesKey
		if err = k.unmarshal(key); err != nil {
			return err
		}
		elected, err := f.isElected(curHeight, k.featureID)
		if err != nil {
			return err
		}
		if elected {
			// Add feature to the list of approved.
			r := &approvedFeaturesRecord{curHeight, curBlockID}
			if err := f.approveFeature(k.featureID, r); err != nil {
				return err
			}
		}
		// Remove feature from the voting list anyway:
		// next voting period starts from scratch.
		f.dbBatch.Delete(key)
	}
	return nil
}

// Update activation list.
func (f *features) activateFeatures(curHeight uint64, curBlockID crypto.Signature) error {
	iter, err := f.db.NewKeyIterator([]byte{approvedFeaturesKeyPrefix})
	if err != nil {
		return err
	}
	defer func() {
		iter.Release()
		if err := iter.Error(); err != nil {
			log.Fatalf("Iterator error: %v", err)
		}
	}()

	for iter.Next() {
		// Iterate approved features.
		var k approvedFeaturesKey
		if err = k.unmarshal(keyvalue.SafeKey(iter)); err != nil {
			return err
		}
		alreadyActivated, err := f.isActivated(k.featureID)
		if err != nil {
			return err
		}
		if alreadyActivated {
			continue
		}
		approvalHeight, err := f.approvalHeight(k.featureID)
		if err != nil {
			return err
		}
		needToActivate := (curHeight - approvalHeight) >= f.settings.ActivationWindowSize(curHeight)
		if needToActivate {
			// Add feature to the list of activated.
			r := &activatedFeaturesRecord{curHeight, curBlockID}
			if err := f.activateFeature(k.featureID, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *features) finishVoting(curHeight uint64, curBlockID crypto.Signature) error {
	if err := f.activateFeatures(curHeight, curBlockID); err != nil {
		return err
	}
	if err := f.approveFeatures(curHeight, curBlockID); err != nil {
		return err
	}
	return nil
}
