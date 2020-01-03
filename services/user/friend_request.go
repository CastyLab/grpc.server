package user

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"movie.night.gRPC.server/db"
	"movie.night.gRPC.server/db/models"
	"movie.night.gRPC.server/proto"
	"movie.night.gRPC.server/services/auth"
	"net/http"
	"time"
)

func (s *Service) SendFriendRequest(ctx context.Context, req *proto.FriendRequest) (*proto.Response, error) {

	var (
		database   = db.Connection
		friend     = new(models.User)
		mCtx, _    = context.WithTimeout(ctx, 20 * time.Second)

		userCollection    = database.Collection("users")
		friendsCollection = database.Collection("friends")

		failedResponse = &proto.Response{
			Status:  "failed",
			Code:    http.StatusInternalServerError,
			Message: "Could not create friend request, Please try again later!",
		}
	)

	user, err := auth.Authenticate(req.AuthRequest)
	if err != nil {
		return &proto.Response{
			Status:  "failed",
			Code:    http.StatusUnauthorized,
			Message: "Unauthorized!",
		}, nil
	}

	objectId, err := primitive.ObjectIDFromHex(string(req.FriendId))
	if err != nil {
		return nil, fmt.Errorf("invalid friend id")
	}

	if err := userCollection.FindOne(mCtx, bson.M{"_id": objectId}).Decode(friend); err != nil {
		return nil, fmt.Errorf("invalid user")
	}

	friendRequest := bson.M{
		"friend_id": friend.ID,
		"user_id":   user.ID,
		"accepted":  false,
	}

	if _, err := friendsCollection.InsertOne(mCtx, friendRequest); err != nil {
		return failedResponse, nil
	}

	return &proto.Response{
		Status:  "success",
		Code:    http.StatusOK,
		Message: "Friend request added successfully!",
	}, nil
}
