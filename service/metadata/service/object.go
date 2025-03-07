package service

/*
import (
	"context"
	"encoding/base64"

	"cosmossdk.io/math"
	"github.com/bnb-chain/greenfield/types/s3util"
	"github.com/bnb-chain/greenfield/x/storage/types"

	"github.com/bnb-chain/greenfield-storage-provider/pkg/log"
	metatypes "github.com/bnb-chain/greenfield-storage-provider/service/metadata/types"
	model "github.com/bnb-chain/greenfield-storage-provider/store/bsdb"
)

// ListObjectsByBucketName list objects info by a bucket name
func (metadata *Metadata) ListObjectsByBucketName(ctx context.Context, req *metatypes.ListObjectsByBucketNameRequest) (resp *metatypes.ListObjectsByBucketNameResponse, err error) {
	var (
		results               []*model.ListObjectsResult
		keyCount              uint64
		isTruncated           bool
		nextContinuationToken string
		maxKeys               uint64
		commonPrefixes        []string
		res                   []*metatypes.Object
	)

	maxKeys = req.MaxKeys
	// if the user does not provide any input parameters, default values will be used
	if req.MaxKeys == 0 {
		maxKeys = model.ListObjectsDefaultMaxKeys
	}

	// returns some or all (up to 1000) of the objects in a bucket with each request
	if req.MaxKeys > model.ListObjectsLimitSize {
		maxKeys = model.ListObjectsLimitSize
	}

	ctx = log.Context(ctx, req)
	results, err = metadata.bsDB.ListObjectsByBucketName(req.BucketName, req.ContinuationToken, req.Prefix, req.Delimiter, int(maxKeys))
	if err != nil {
		log.CtxErrorw(ctx, "failed to list objects by bucket name", "error", err)
		return
	}

	for i, object := range results {
		// Avoid returning the extra queried continuation value to the user.
		if i == int(maxKeys) {
			break
		}
		if object.ResultType == "common_prefix" {
			commonPrefixes = append(commonPrefixes, object.PathName)
		} else {
			res = append(res, &metatypes.Object{
				ObjectInfo: &types.ObjectInfo{
					Owner:                object.Owner.String(),
					BucketName:           object.BucketName,
					ObjectName:           object.ObjectName,
					Id:                   math.NewUintFromBigInt(object.ObjectID.Big()),
					PayloadSize:          object.PayloadSize,
					ContentType:          object.ContentType,
					CreateAt:             object.CreateTime,
					ObjectStatus:         types.ObjectStatus(types.ObjectStatus_value[object.ObjectStatus]),
					RedundancyType:       types.RedundancyType(types.RedundancyType_value[object.RedundancyType]),
					SourceType:           types.SourceType(types.SourceType_value[object.SourceType]),
					Checksums:            object.Checksums,
					SecondarySpAddresses: object.SecondarySpAddresses,
					Visibility:           types.VisibilityType(types.VisibilityType_value[object.Visibility]),
				},
				LockedBalance: object.LockedBalance.String(),
				Removed:       object.Removed,
				UpdateAt:      object.UpdateAt,
				DeleteAt:      object.DeleteAt,
				DeleteReason:  object.DeleteReason,
				Operator:      object.Operator.String(),
				CreateTxHash:  object.CreateTxHash.String(),
				UpdateTxHash:  object.UpdateTxHash.String(),
				SealTxHash:    object.SealTxHash.String(),
			})
		}
	}

	keyCount = uint64(len(results))
	// if keyCount is equal to req.MaxKeys+1 which means that we additionally return NextContinuationToken, and it is not counted in the keyCount
	// isTruncated set to false if all the results were returned, set to true if more keys are available to return
	// remove the returned NextContinuationToken object and separately return its object ID to the user for the next API call
	if keyCount == req.MaxKeys+1 {
		isTruncated = true
		keyCount -= 1
		nextContinuationToken = results[len(results)-1].PathName
		if req.Delimiter == "" {
			nextContinuationToken = results[len(results)-1].ObjectName
		}
	}

	resp = &metatypes.ListObjectsByBucketNameResponse{
		Objects:               res,
		KeyCount:              keyCount,
		MaxKeys:               maxKeys,
		IsTruncated:           isTruncated,
		NextContinuationToken: base64.StdEncoding.EncodeToString([]byte(nextContinuationToken)),
		Name:                  req.BucketName,
		Prefix:                req.Prefix,
		Delimiter:             req.Delimiter,
		CommonPrefixes:        commonPrefixes,
		ContinuationToken:     base64.StdEncoding.EncodeToString([]byte(req.ContinuationToken)),
	}
	log.CtxInfo(ctx, "succeed to list objects by bucket name")
	return resp, nil
}

// ListDeletedObjectsByBlockNumberRange list deleted objects info by a block number range
func (metadata *Metadata) ListDeletedObjectsByBlockNumberRange(ctx context.Context, req *metatypes.ListDeletedObjectsByBlockNumberRangeRequest) (resp *metatypes.ListDeletedObjectsByBlockNumberRangeResponse, err error) {
	ctx = log.Context(ctx, req)

	endBlockNumber, err := metadata.bsDB.GetLatestBlockNumber()
	if err != nil {
		log.CtxErrorw(ctx, "failed to get the latest block number", "error", err)
		return nil, err
	}

	if endBlockNumber > req.EndBlockNumber {
		endBlockNumber = req.EndBlockNumber
	}

	objects, err := metadata.bsDB.ListDeletedObjectsByBlockNumberRange(req.StartBlockNumber, endBlockNumber, req.IsFullList)
	if err != nil {
		log.CtxErrorw(ctx, "failed to list deleted objects by block number range", "error", err)
		return nil, err
	}

	res := make([]*metatypes.Object, 0)
	for _, object := range objects {
		res = append(res, &metatypes.Object{
			ObjectInfo: &types.ObjectInfo{
				Owner:                object.Owner.String(),
				BucketName:           object.BucketName,
				ObjectName:           object.ObjectName,
				Id:                   math.NewUintFromBigInt(object.ObjectID.Big()),
				PayloadSize:          object.PayloadSize,
				ContentType:          object.ContentType,
				CreateAt:             object.CreateTime,
				ObjectStatus:         types.ObjectStatus(types.ObjectStatus_value[object.ObjectStatus]),
				RedundancyType:       types.RedundancyType(types.RedundancyType_value[object.RedundancyType]),
				SourceType:           types.SourceType(types.SourceType_value[object.SourceType]),
				Checksums:            object.Checksums,
				SecondarySpAddresses: object.SecondarySpAddresses,
				Visibility:           types.VisibilityType(types.VisibilityType_value[object.Visibility]),
			},
			LockedBalance: object.LockedBalance.String(),
			Removed:       object.Removed,
			UpdateAt:      object.UpdateAt,
			DeleteAt:      object.DeleteAt,
			DeleteReason:  object.DeleteReason,
			Operator:      object.Operator.String(),
			CreateTxHash:  object.CreateTxHash.String(),
			UpdateTxHash:  object.UpdateTxHash.String(),
			SealTxHash:    object.SealTxHash.String(),
		})
	}

	resp = &metatypes.ListDeletedObjectsByBlockNumberRangeResponse{
		Objects:        res,
		EndBlockNumber: endBlockNumber,
	}
	log.CtxInfow(ctx, "succeed to list deleted objects by block number range")
	return resp, nil
}

// GetObjectMeta get object metadata
func (metadata *Metadata) GetObjectMeta(ctx context.Context, req *metatypes.GetObjectMetaRequest) (resp *metatypes.GetObjectMetaResponse, err error) {
	var (
		object *model.Object
		res    *metatypes.Object
	)

	ctx = log.Context(ctx, req)
	if err = s3util.CheckValidObjectName(req.ObjectName); err != nil {
		log.Errorw("failed to check object name", "object_name", req.ObjectName, "error", err)
		return nil, err
	}

	object, err = metadata.bsDB.GetObjectByName(req.ObjectName, req.BucketName, req.IsFullList)
	if err != nil {
		log.CtxErrorw(ctx, "failed to get object by object name", "error", err)
		return nil, err
	}

	if object != nil {
		res = &metatypes.Object{
			ObjectInfo: &types.ObjectInfo{
				Owner:                object.Owner.String(),
				BucketName:           object.BucketName,
				ObjectName:           object.ObjectName,
				Id:                   math.NewUintFromBigInt(object.ObjectID.Big()),
				PayloadSize:          object.PayloadSize,
				ContentType:          object.ContentType,
				CreateAt:             object.CreateTime,
				ObjectStatus:         types.ObjectStatus(types.ObjectStatus_value[object.ObjectStatus]),
				RedundancyType:       types.RedundancyType(types.RedundancyType_value[object.RedundancyType]),
				SourceType:           types.SourceType(types.SourceType_value[object.SourceType]),
				Checksums:            object.Checksums,
				SecondarySpAddresses: object.SecondarySpAddresses,
				Visibility:           types.VisibilityType(types.VisibilityType_value[object.Visibility]),
			},
			LockedBalance: object.LockedBalance.String(),
			Removed:       object.Removed,
			DeleteAt:      object.DeleteAt,
			DeleteReason:  object.DeleteReason,
			Operator:      object.Operator.String(),
			CreateTxHash:  object.CreateTxHash.String(),
			UpdateTxHash:  object.UpdateTxHash.String(),
			SealTxHash:    object.SealTxHash.String(),
		}
	}
	resp = &metatypes.GetObjectMetaResponse{Object: res}
	log.CtxInfo(ctx, "succeed to get object meta")
	return resp, nil
}

*/
