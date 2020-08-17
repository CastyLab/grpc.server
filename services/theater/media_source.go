package theater

import (
	"context"
	"fmt"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.server/db"
	"github.com/CastyLab/grpc.server/db/models"
	"github.com/CastyLab/grpc.server/helpers"
	"github.com/CastyLab/grpc.server/internal"
	"github.com/CastyLab/grpc.server/services"
	"github.com/CastyLab/grpc.server/services/auth"
	"github.com/getsentry/sentry-go"
	"github.com/golang/protobuf/ptypes/any"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"net/http"
	"os"
	"time"
)

func (s *Service) SelectMediaSource(ctx context.Context, req *proto.MediaSourceAuthRequest) (*proto.Response, error) {

	var (
		database   = db.Connection
		collection = database.Collection("theaters")
		emptyResponse = status.Error(codes.Aborted, "Could not update theater!")
	)

	user, err := auth.Authenticate(req.AuthRequest)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Unauthorized!")
	}

	var (
		theater = new(models.Theater)
		findTheater = bson.M{ "user_id": user.ID }
	)

	if err := db.Connection.Collection("theaters").FindOne(ctx, findTheater).Decode(theater); err != nil {
		return nil, status.Error(codes.NotFound, "Could not find theater!")
	}

	mediaSourceId, err := primitive.ObjectIDFromHex(req.Media.Id)
	if err != nil {
		return nil, err
	}

	count, err := database.Collection("media_sources").CountDocuments(ctx, bson.M{
		"_id":     mediaSourceId,
		"user_id": user.ID,
	})

	if err != nil {
		return nil, err
	}

	if count != 0 {

		_, err := collection.UpdateOne(ctx, bson.M{ "user_id": user.ID }, bson.M{
			"$set": bson.M{
				"media_source_id": mediaSourceId,
			},
		})
		if err != nil {
			return nil, err
		}

		// sending new media source to websocket
		err = internal.Client.TheaterService.SendMediaSourceUpdateEvent(mediaSourceId.Hex(), theater.ID.Hex())
		if err != nil {
			sentry.CaptureException(err)
		}

		return &proto.Response{
			Status:  "success",
			Code:    http.StatusOK,
			Message: "Media source selected successfully!",
		}, nil
	}

	return nil, emptyResponse
}

func (s *Service) SavePosterFromUrl(url string) (string, error) {
	var (
		storagePath = os.Getenv("STORAGE_PATH")
		posterName  = services.RandomNumber(20)
	)
	avatarFile, err := os.Create(fmt.Sprintf("%s/uploads/posters/%s.png", storagePath, posterName))
	if err != nil {
		return posterName, err
	}
	defer avatarFile.Close()
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return posterName, err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(avatarFile, resp.Body); err != nil {
		return posterName, err
	}
	return posterName, nil
}

func (s *Service) AddMediaSource(ctx context.Context, req *proto.MediaSourceAuthRequest) (*proto.Response, error) {

	var (
		validationErrors []*any.Any
		database   = db.Connection
		collection = database.Collection("media_sources")
		theatersCollection = database.Collection("theaters")
		failedResponse = status.Error(codes.Internal, "Could not add a new media source. Please try agian later!")
	)

	user, err := auth.Authenticate(req.AuthRequest)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Unauthorized!")
	}

	var (
		theater = new(models.Theater)
		findTheater = bson.M{ "user_id": user.ID }
	)

	if err := db.Connection.Collection("theaters").FindOne(ctx, findTheater).Decode(theater); err != nil {
		return nil, status.Error(codes.NotFound, "Could not find theater!")
	}

	if req.Media.Uri == "" {
		validationErrors = append(validationErrors, &any.Any{
			TypeUrl: "uri",
			Value: []byte("Uri is required!"),
		})
	}

	if req.Media.Type == proto.MediaSource_UNKNOWN {
		validationErrors = append(validationErrors, &any.Any{
			TypeUrl: "type",
			Value: []byte("Media source type can not be unknown!"),
		})
	}

	if len(validationErrors) > 0 {
		return nil, status.ErrorProto(&spb.Status{
			Code: int32(codes.InvalidArgument),
			Message: "Validation Error!",
			Details: validationErrors,
		})
	}

	var poster string
	poster, err = s.SavePosterFromUrl(req.Media.Banner)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("could not upload poster %v", err))
		poster = "default"
	}

	mediaSource := bson.M{
		"title":              req.Media.Title,
		"type":               req.Media.Type,
		"banner":             poster,
		"uri":                req.Media.Uri,
		"length":             req.Media.Length,
		"user_id":            user.ID,
		"created_at":         time.Now(),
		"updated_at":         time.Now(),
	}

	result, err := collection.InsertOne(ctx, mediaSource)
	if err != nil {
		return nil, failedResponse
	}

	insertedID := result.InsertedID.(primitive.ObjectID)
	update, _ := theatersCollection.UpdateOne(ctx, bson.M{"user_id": user.ID}, bson.M{
		"$set": bson.M{
			"media_source_id": insertedID,
		},
	})

	if update.ModifiedCount == 0 {
		return nil, status.Errorf(codes.Internal, "could not update media source, please try again later!")
	}

	// sending new media source to websocket
	err = internal.Client.TheaterService.SendMediaSourceUpdateEvent(insertedID.Hex(), theater.ID.Hex())
	if err != nil {
		sentry.CaptureException(err)
	}

	return &proto.Response{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Media source created successfully!",
	}, nil
}

func (s *Service) GetMediaSource(ctx context.Context, req *proto.MediaSourceAuthRequest) (*proto.TheaterMediaSourcesResponse, error) {

	var (
		database     = db.Connection
		mediaSources = make([]*proto.MediaSource, 0)
	)

	user, err := auth.Authenticate(req.AuthRequest)
	if err != nil {
		return &proto.TheaterMediaSourcesResponse{
			Status:  "failed",
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized!",
		}, nil
	}

	mediaSourceObjectId, err := primitive.ObjectIDFromHex(req.Media.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "MediaSourceId is invalid!")
	}

	var (
		mediaSource = new(models.MediaSource)
		filter = bson.M{
			"user_id": user.ID,
			"_id": mediaSourceObjectId,
		}
	)

	if err := database.Collection("media_sources").FindOne(ctx, filter).Decode(mediaSource); err != nil {
		return nil, status.Error(codes.NotFound, "Could not find media source!")
	}

	protoMediaSource, err := helpers.NewMediaSourceProto(mediaSource)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not parse media source!")
	}
	mediaSources = append(mediaSources, protoMediaSource)

	return &proto.TheaterMediaSourcesResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Result:  mediaSources,
	}, nil
}

func (s *Service) GetMediaSources(ctx context.Context, req *proto.MediaSourceAuthRequest) (*proto.TheaterMediaSourcesResponse, error) {

	var (
		database     = db.Connection
		mediaSources = make([]*proto.MediaSource, 0)
	)

	user, err := auth.Authenticate(req.AuthRequest)
	if err != nil {
		return &proto.TheaterMediaSourcesResponse{
			Status:  "failed",
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized!",
		}, nil
	}

	cursor, err := database.Collection("media_sources").Find(ctx, bson.M{ "user_id": user.ID })
	if err != nil {
		return nil, status.Error(codes.NotFound, "Could not find theater!")
	}

	for cursor.Next(ctx) {
		dbMediaSource := new(models.MediaSource)
		if err := cursor.Decode(dbMediaSource); err != nil {
			continue
		}
		protoMediaSource, err := helpers.NewMediaSourceProto(dbMediaSource)
		if err != nil {
			continue
		}
		mediaSources = append(mediaSources, protoMediaSource)
	}

	return &proto.TheaterMediaSourcesResponse{
		Status:  "success",
		Code:    http.StatusOK,
		Result:  mediaSources,
	}, nil
}

func (s *Service) RemoveMediaSource(ctx context.Context, req *proto.MediaSourceRemoveRequest) (*proto.Response, error) {

	collection := db.Connection.Collection("media_sources")

	user, err := auth.Authenticate(req.AuthRequest)
	if err != nil {
		return &proto.Response{
			Status:  "failed",
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized!",
		}, nil
	}

	mediaSourceObjectID, err := primitive.ObjectIDFromHex(req.MediaSourceId)
	if err != nil {
		return nil, status.Error(codes.Internal, "Could not parse MediaSourceId!")
	}

	var (
		theater = new(models.Theater)
		findTheater = bson.M{ "user_id": user.ID }
	)

	if err := db.Connection.Collection("theaters").FindOne(ctx, findTheater).Decode(theater); err != nil {
		return nil, status.Error(codes.Internal, "Could not find theater!")
	}

	if result, err := collection.DeleteOne(ctx, bson.M{ "_id": mediaSourceObjectID, "user_id": user.ID }); err == nil {
		if result.DeletedCount == 1 {

			// sending new media source to websocket
			err := internal.Client.TheaterService.SendMediaSourceUpdateEvent(req.MediaSourceId, theater.ID.Hex())
			if err != nil {
				sentry.CaptureException(err)
			}

			return &proto.Response{
				Status:  "success",
				Code:    http.StatusOK,
				Message: "Media source deleted successfully@",
			}, nil
		}
	}

	return nil, status.Error(codes.Aborted, "Could not delete media source. Please try again later!")
}