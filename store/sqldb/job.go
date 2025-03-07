package sqldb

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"cosmossdk.io/math"
	"gorm.io/gorm"

	servicetypes "github.com/bnb-chain/greenfield-storage-provider/service/types"
	"github.com/bnb-chain/greenfield-storage-provider/util"
	storagetypes "github.com/bnb-chain/greenfield/x/storage/types"
)

// CreateUploadJob create JobTable record and ObjectTable record; use JobID field for association
func (s *SpDBImpl) CreateUploadJob(objectInfo *storagetypes.ObjectInfo) (*servicetypes.JobContext, error) {
	insertJobRecord := &JobTable{
		JobType:      int32(servicetypes.JobType_JOB_TYPE_UPLOAD_OBJECT),
		JobState:     int32(servicetypes.JobState_JOB_STATE_INIT_UNSPECIFIED),
		CreatedTime:  time.Now(),
		ModifiedTime: time.Now(),
	}
	result := s.db.Create(insertJobRecord)
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, fmt.Errorf("failed to insert job table: %s", result.Error)
	}

	insertObjectRecord := &ObjectTable{
		ObjectID:             objectInfo.Id.Uint64(),
		JobID:                insertJobRecord.JobID,
		Owner:                objectInfo.GetOwner(),
		BucketName:           objectInfo.GetBucketName(),
		ObjectName:           objectInfo.GetObjectName(),
		PayloadSize:          objectInfo.GetPayloadSize(),
		Visibility:           int32(objectInfo.GetVisibility()),
		ContentType:          objectInfo.GetContentType(),
		CreatedAtHeight:      objectInfo.GetCreateAt(),
		ObjectStatus:         int32(objectInfo.GetObjectStatus()),
		RedundancyType:       int32(objectInfo.GetRedundancyType()),
		SourceType:           int32(objectInfo.GetSourceType()),
		SpIntegrityHash:      util.BytesSliceToString(objectInfo.GetChecksums()),
		SecondarySpAddresses: util.JoinWithComma(objectInfo.GetSecondarySpAddresses()),
	}
	result = s.db.Create(insertObjectRecord)
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, fmt.Errorf("failed to insert object table: %s", result.Error)
	}

	return &servicetypes.JobContext{
		JobId:        insertJobRecord.JobID,
		JobType:      servicetypes.JobType(insertJobRecord.JobType),
		JobState:     servicetypes.JobState(insertJobRecord.JobState),
		JobErrorCode: insertJobRecord.JobErrorCode,
		CreateTime:   insertJobRecord.CreatedTime.Unix(),
		ModifyTime:   insertJobRecord.ModifiedTime.Unix(),
	}, nil
}

// UpdateJobState update JobTable record's state
func (s *SpDBImpl) UpdateJobState(objectID uint64, state servicetypes.JobState) error {
	queryObjectReturn := &ObjectTable{}
	result := s.db.First(queryObjectReturn, "object_id = ?", objectID)
	if result.Error != nil {
		return fmt.Errorf("failed to query object table: %s", result.Error)
	}
	queryCondition := &JobTable{
		JobID: queryObjectReturn.JobID,
	}
	updateFields := &JobTable{
		JobState:     int32(state),
		ModifiedTime: time.Now(),
	}
	result = s.db.Model(queryCondition).Updates(updateFields)
	if result.Error != nil || result.RowsAffected != 1 {
		return fmt.Errorf("failed to update job record's state: %s", result.Error)
	}
	return nil
}

// GetJobByID query JobTable by jobID and convert to service/types.JobContext
func (s *SpDBImpl) GetJobByID(jobID uint64) (*servicetypes.JobContext, error) {
	queryReturn := &JobTable{}
	result := s.db.First(queryReturn, "job_id = ?", jobID)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query job table: %s", result.Error)
	}
	return &servicetypes.JobContext{
		JobId:        queryReturn.JobID,
		JobType:      servicetypes.JobType(queryReturn.JobType),
		JobState:     servicetypes.JobState(queryReturn.JobState),
		JobErrorCode: queryReturn.JobErrorCode,
		CreateTime:   queryReturn.CreatedTime.Unix(),
		ModifyTime:   queryReturn.ModifiedTime.Unix(),
	}, nil
}

// GetJobByObjectID query JobTable by jobID and convert to service/types.JobContext
func (s *SpDBImpl) GetJobByObjectID(objectID uint64) (*servicetypes.JobContext, error) {
	queryReturn := &ObjectTable{}
	result := s.db.First(queryReturn, "object_id = ?", objectID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query object table: %s", result.Error)
	}
	jobQueryReturn := &JobTable{}
	result = s.db.First(jobQueryReturn, "job_id = ?", queryReturn.JobID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query job table: %s", result.Error)
	}
	return &servicetypes.JobContext{
		JobId:        jobQueryReturn.JobID,
		JobType:      servicetypes.JobType(jobQueryReturn.JobType),
		JobState:     servicetypes.JobState(jobQueryReturn.JobState),
		JobErrorCode: jobQueryReturn.JobErrorCode,
		CreateTime:   jobQueryReturn.CreatedTime.Unix(),
		ModifyTime:   jobQueryReturn.ModifiedTime.Unix(),
	}, nil
}

// GetObjectInfo query ObjectTable by objectID and convert to storage/types.ObjectInfo.
func (s *SpDBImpl) GetObjectInfo(objectID uint64) (*storagetypes.ObjectInfo, error) {
	queryReturn := &ObjectTable{}
	result := s.db.First(queryReturn, "object_id = ?", objectID)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query object table: %s", result.Error)
	}
	checksums, err := util.StringToBytesSlice(queryReturn.SpIntegrityHash)
	if err != nil {
		return nil, err
	}
	return &storagetypes.ObjectInfo{
		Owner:                queryReturn.Owner,
		BucketName:           queryReturn.BucketName,
		ObjectName:           queryReturn.ObjectName,
		Id:                   math.NewUint(queryReturn.ObjectID),
		PayloadSize:          queryReturn.PayloadSize,
		Visibility:           storagetypes.VisibilityType(queryReturn.Visibility),
		ContentType:          queryReturn.ContentType,
		CreateAt:             queryReturn.CreatedAtHeight,
		ObjectStatus:         storagetypes.ObjectStatus(queryReturn.ObjectStatus),
		RedundancyType:       storagetypes.RedundancyType(queryReturn.RedundancyType),
		SourceType:           storagetypes.SourceType(queryReturn.SourceType),
		Checksums:            checksums,
		SecondarySpAddresses: strings.Split(queryReturn.SecondarySpAddresses, ","),
	}, nil
}

// SetObjectInfo set ObjectTable's record by objectID
func (s *SpDBImpl) SetObjectInfo(objectID uint64, objectInfo *storagetypes.ObjectInfo) error {
	queryReturn := &ObjectTable{}
	result := s.db.First(queryReturn, "object_id = ?", objectID)
	isNotFound := errors.Is(result.Error, gorm.ErrRecordNotFound)
	if result.Error != nil && !isNotFound {
		return fmt.Errorf("failed to query object table: %s", result.Error)
	}

	updateFields := &ObjectTable{
		ObjectID:             objectID,
		Owner:                objectInfo.GetOwner(),
		BucketName:           objectInfo.GetBucketName(),
		ObjectName:           objectInfo.GetObjectName(),
		PayloadSize:          objectInfo.GetPayloadSize(),
		Visibility:           int32(objectInfo.GetVisibility()),
		ContentType:          objectInfo.GetContentType(),
		CreatedAtHeight:      objectInfo.GetCreateAt(),
		ObjectStatus:         int32(objectInfo.GetObjectStatus()),
		RedundancyType:       int32(objectInfo.GetRedundancyType()),
		SourceType:           int32(objectInfo.GetSourceType()),
		SpIntegrityHash:      util.BytesSliceToString(objectInfo.GetChecksums()),
		SecondarySpAddresses: util.JoinWithComma(objectInfo.GetSecondarySpAddresses()),
	}
	if isNotFound {
		// if record is not found, insert a new record
		insertJobRecord := &JobTable{
			JobType:      int32(servicetypes.JobType_JOB_TYPE_UPLOAD_OBJECT),
			JobState:     int32(servicetypes.JobState_JOB_STATE_INIT_UNSPECIFIED),
			CreatedTime:  time.Now(),
			ModifiedTime: time.Now(),
		}
		result = s.db.Create(insertJobRecord)
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("failed to insert job table: %s", result.Error)
		}
		updateFields.JobID = insertJobRecord.JobID
		result = s.db.Create(updateFields)
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("failed to insert object table: %s", result.Error)
		}
	} else {
		// if record exists, update record
		queryCondition := &ObjectTable{ObjectID: objectID}
		updateFields.JobID = queryReturn.JobID
		result = s.db.Model(queryCondition).Updates(updateFields)
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("failed to update object table: %s", result.Error)
		}
	}
	return nil
}
