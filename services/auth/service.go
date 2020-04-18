package auth

import (
	"context"
	"github.com/CastyLab/grpc.proto/proto"
	"github.com/CastyLab/grpc.server/db"
	"github.com/CastyLab/grpc.server/db/models"
	"github.com/CastyLab/grpc.server/helpers"
	"github.com/CastyLab/grpc.server/jwt"
	"github.com/getsentry/sentry-go"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
	"net/http"
	"regexp"
	"time"
)

type Service struct {}

func (s *Service) isEmail(user string) bool {

	re := regexp.MustCompile(
		"^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])" +
			"?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	if re.MatchString(user) {
		return true
	}

	return false
}

func (s *Service) validatePassword(user *models.User, pass string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pass))
	return err == nil
}

func (s *Service) Authenticate(ctx context.Context, req *proto.AuthRequest) (*proto.AuthResponse, error) {

	var (
		collection   = db.Connection.Collection("users")
		user         = new(models.User)
		mCtx, _      = context.WithTimeout(ctx, 10 * time.Second)
		unauthorized = status.Error(codes.Unauthenticated, "Unauthorized!")
	)

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "Captcha is required!")
	}

	recaptcha := md.Get("g-recaptcha-response")
	if success, err := helpers.VerifyRecaptcha(recaptcha[0]); err != nil || !success {
		log.Println(req, err)
		return nil, status.Error(codes.InvalidArgument, "Captcha is required!")
	}

	if req.User == "" {
		return nil, unauthorized
	}

	if req.Pass == "" {
		return nil, unauthorized
	}

	filter := bson.M{ "username": req.User }
	if s.isEmail(req.User) {
		filter = bson.M{ "email": req.User }
	}

	if err := collection.FindOne(mCtx, filter).Decode(&user); err != nil {
		return nil, status.Error(codes.NotFound, "Could not find user!")
	}

	if s.validatePassword(user, req.Pass) {

		token, refreshedToken, err := jwt.CreateNewTokens(mCtx, user.ID.Hex())
		if err != nil {
			sentry.CaptureException(err)
			return nil, status.Error(codes.Internal, "Could not create auth token, Please try again later!")
		}

		return &proto.AuthResponse{
			Status: "success",
			Code:   http.StatusOK,
			Token:  []byte(token),
			RefreshedToken:  []byte(refreshedToken),
		}, nil
	}

	return nil, unauthorized
}

