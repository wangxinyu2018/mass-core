package ldb

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/wangxinyu2018/mass-core/database/storage"
	"github.com/wangxinyu2018/mass-core/interfaces"
	"github.com/wangxinyu2018/mass-core/massutil"
	"github.com/wangxinyu2018/mass-core/wire"
)

var (
	punishmentPrefix = []byte("PUNISH")
)

func punishmentPubKeyToKey(pk interfaces.PublicKey) ([]byte, error) {
	keyBytes := pk.SerializeCompressed()
	keyBytesLen := len(keyBytes)
	switch keyBytesLen {
	case PUBLICKEYLENGTH_MASS, PUBLICKEYLENGTH_CHIA:
	default:
		return nil, fmt.Errorf("invalid pk length %d", keyBytesLen)
	}
	key := make([]byte, 6+keyBytesLen)
	copy(key, punishmentPrefix)
	copy(key[6:], keyBytes[:])
	return key, nil
}

func insertPunishmentAtomic(batch storage.Batch, fpk *wire.FaultPubKey) error {
	key, err := punishmentPubKeyToKey(fpk.PubKey)
	if err != nil {
		return err
	}
	data, err := fpk.Bytes(wire.DB)
	if err != nil {
		return err
	}
	return batch.Put(key, data)
}

func insertPunishments(batch storage.Batch, fpks []*wire.FaultPubKey) error {
	for _, fpk := range fpks {
		err := insertPunishmentAtomic(batch, fpk)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *ChainDb) InsertPunishment(fpk *wire.FaultPubKey) error {
	key, err := punishmentPubKeyToKey(fpk.PubKey)
	if err != nil {
		return err
	}
	data, err := fpk.Bytes(wire.DB)
	if err != nil {
		return err
	}
	return db.stor.Put(key, data)
}

func dropPunishments(batch storage.Batch, pks []*wire.FaultPubKey) error {
	for _, pk := range pks {
		key, err := punishmentPubKeyToKey(pk.PubKey)
		if err != nil {
			return err
		}
		batch.Delete(key)
	}
	return nil
}

func (db *ChainDb) ExistsPunishment(pk interfaces.PublicKey) (bool, error) {
	key, err := punishmentPubKeyToKey(pk)
	if err != nil {
		return false, err
	}
	return db.stor.Has(key)
}

func (db *ChainDb) FetchAllPunishment() ([]*wire.FaultPubKey, error) {
	res := make([]*wire.FaultPubKey, 0)
	iter := db.stor.NewIterator(storage.BytesPrefix(punishmentPrefix))
	defer iter.Release()

	for iter.Next() {
		fpk, err := wire.NewFaultPubKeyFromBytes(iter.Value(), wire.DB)
		if err != nil {
			return nil, err
		}
		res = append(res, fpk)
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return res, nil
}

func insertBlockPunishments(batch storage.Batch, blk *massutil.Block) error {
	faultPks := blk.MsgBlock().Proposals.PunishmentArea
	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[0:2], uint16(len(faultPks)))

	var shaListData bytes.Buffer
	shaListData.Write(b2[:])

	for _, fpk := range faultPks {
		sha := wire.DoubleHashH(fpk.PubKey.SerializeUncompressed())
		shaListData.Write(sha.Bytes())
		err := insertFaultPk(batch, blk.Height(), fpk, &sha)
		if err != nil {
			return err
		}

		// table - PUNISH
		key, err := punishmentPubKeyToKey(fpk.PubKey)
		if err != nil {
			return err
		}
		batch.Delete(key)
	}

	// table - BANHGT
	if len(faultPks) > 0 {
		heightIndex := faultPkHeightToKey(blk.Height())
		if err := batch.Put(heightIndex, shaListData.Bytes()); err != nil {
			return err
		}
	}
	return nil
}
