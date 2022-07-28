/*---------------------------------------------------------------------------------------------
 *  Copyright (c) IBAX. All rights reserved.
 *  See LICENSE in the project root for license information.
 *--------------------------------------------------------------------------------------------*/

package sqldb

import (
	"github.com/IBAX-io/go-ibax/packages/consts"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SpentInfo is model
type SpentInfo struct {
	InputTxHash  []byte `gorm:"default:(-)"`
	InputIndex   int32
	OutputTxHash []byte `gorm:"not null"`
	OutputIndex  int32  `gorm:"not null"`
	OutputKeyId  int64  `gorm:"not null"`
	OutputValue  string `gorm:"not null"`
	Scene        string
	Ecosystem    int64
	Contract     string
	BlockId      int64
	Asset        string
	Action       string `gorm:"-"` // UTXO operation control : change
}

// TableName returns name of table
func (si *SpentInfo) TableName() string {
	return "spent_info"
}

// CreateSpentInfoBatches is creating record of model
func CreateSpentInfoBatches(dbTx *gorm.DB, spentInfos []SpentInfo) error {
	//for _, info := range spentInfos {
	//	fmt.Println(hex.EncodeToString(info.InputTxHash), info.InputIndex, hex.EncodeToString(info.OutputTxHash), info.OutputIndex, info.OutputKeyId, info.OutputValue, info.BlockId)
	//}

	return dbTx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "output_tx_hash"}, {Name: "output_key_id"}, {Name: "output_index"}},
		DoUpdates: clause.AssignmentColumns([]string{"input_tx_hash", "input_index"}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: "spent_info.output_tx_hash", Value: gorm.Expr(`"excluded"."output_tx_hash"`)},
			clause.Eq{Column: "spent_info.output_key_id", Value: gorm.Expr(`"excluded"."output_key_id"`)},
			clause.Eq{Column: "spent_info.output_index", Value: gorm.Expr(`"excluded"."output_index"`)},
		}},
	}).CreateInBatches(spentInfos, 1000).Error
}

func GetTxOutputsEcosystem(db *DbTransaction, ecosystem int64, keyIds []int64) ([]SpentInfo, error) {
	query :=
		` SELECT si.output_tx_hash, si.output_index, si.output_key_id, si.output_value, si.scene, si.ecosystem, si.contract, si.block_id, si.asset
		FROM spent_info si LEFT JOIN log_transactions AS tr ON si.output_tx_hash = tr.hash
		WHERE si.ecosystem = ? AND si.output_key_id IN ? AND  si.input_tx_hash IS NULL
		ORDER BY si.output_key_id, si.block_id ASC, tr.timestamp ASC `
	var result []SpentInfo
	err := GetDB(db).Raw(query, ecosystem, keyIds).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetTxOutputs(db *DbTransaction, keyIds []int64) ([]SpentInfo, error) {
	query :=
		` SELECT si.output_tx_hash, si.output_index, si.output_key_id, si.output_value, si.scene, si.ecosystem, si.contract, si.block_id, si.asset
		FROM spent_info si LEFT JOIN log_transactions AS tr ON si.output_tx_hash = tr.hash
		WHERE si.output_key_id IN ? AND si.input_tx_hash IS NULL
		ORDER BY si.output_key_id, si.block_id ASC, tr.timestamp ASC `
	var result []SpentInfo
	err := GetDB(db).Raw(query, keyIds).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func RollbackOutputs(blockID int64, db *DbTransaction, logger *log.Entry) error {
	err := GetDB(db).Exec(`UPDATE spent_info SET  input_tx_hash= null , input_index=0 WHERE input_tx_hash  in ( SELECT output_tx_hash FROM "spent_info"  WHERE block_id = ? )`, blockID).Error
	if err != nil {
		logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Errorf("updating input_tx_hash rollback outputs by blockID : %d", blockID)
		return err
	}

	err = GetDB(db).Exec(`DELETE FROM spent_info WHERE block_id = ? `, blockID).Error
	if err != nil {
		logger.WithFields(log.Fields{"type": consts.DBError, "error": err}).Errorf("deleting rollback outputs by blockID : %d", blockID)
		return err
	}

	return nil
}

func GetBlockOutputs(dbTx *DbTransaction, blockID int64) ([]SpentInfo, error) {
	var result []SpentInfo
	err := GetDB(dbTx).Where("block_id = ?", blockID).Find(&result).Error
	return result, err
}