package transport

import (
	"context"
	"errors"
	"time"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/tracing/opentracing"
	grpctransport "github.com/go-kit/kit/transport/grpc"
	jujuratelimit "github.com/juju/ratelimit"
	"github.com/laidingqing/dabanshan/pb"
	o_endpoint "github.com/laidingqing/dabanshan/svcs/order/endpoint"
	m_order "github.com/laidingqing/dabanshan/svcs/order/model"
	"github.com/laidingqing/dabanshan/svcs/order/service"
	stdopentracing "github.com/opentracing/opentracing-go"
	"github.com/sony/gobreaker"
	oldcontext "golang.org/x/net/context"
	"google.golang.org/grpc"
)

type grpcServer struct {
	createOrder grpctransport.Handler
	getOrders   grpctransport.Handler
}

// NewGRPCServer ...
func NewGRPCServer(endpoints o_endpoint.Set, tracer stdopentracing.Tracer, logger log.Logger) pb.OrderRpcServiceServer {
	options := []grpctransport.ServerOption{
		grpctransport.ServerErrorLogger(logger),
	}
	return &grpcServer{
		createOrder: grpctransport.NewServer(
			endpoints.CreateOrderEndpoint,
			decodeGRPCCreateOrderRequest,
			encodeGRPCCreateOrderResponse,
			append(options, grpctransport.ServerBefore(opentracing.GRPCToContext(tracer, "CreateOrder", logger)))...,
		),
		getOrders: grpctransport.NewServer(
			endpoints.GetOrdersEndpoint,
			decodeGRPCGetOrdersRequest,
			encodeGRPCGetOrdersResponse,
			append(options, grpctransport.ServerBefore(opentracing.GRPCToContext(tracer, "GetOrders", logger)))...,
		),
	}
}

// GetUser RPC
func (s *grpcServer) CreateOrder(ctx oldcontext.Context, req *pb.CreateOrderRequest) (*pb.CreatedOrderResponse, error) {
	_, rep, err := s.createOrder.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	res := rep.(*pb.CreatedOrderResponse)
	return res, nil
}

func decodeGRPCCreateOrderRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.CreateOrderRequest)
	return m_order.CreateOrderRequest{
		Amount: float32(req.Amount),
	}, nil
}

func encodeGRPCCreateOrderResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(m_order.CreatedOrderResponse)
	return &pb.CreatedOrderResponse{Err: err2str(resp.Err)}, nil
}

// GetOrders

func (s *grpcServer) GetOrders(ctx oldcontext.Context, req *pb.GetOrdersRequest) (*pb.GetOrdersResponse, error) {
	_, rep, err := s.getOrders.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	res := rep.(*pb.GetOrdersResponse)
	return res, nil
}

func decodeGRPCGetOrdersRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*pb.GetOrdersRequest)
	return m_order.GetOrdersRequest{
		UserID: req.Userid,
	}, nil
}

func encodeGRPCGetOrdersResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(m_order.GetOrdersResponse)
	return &pb.GetOrdersResponse{
		Err: err2str(resp.Err),
	}, nil
}

// NewGRPCClient ...
func NewGRPCClient(conn *grpc.ClientConn, tracer stdopentracing.Tracer, logger log.Logger) service.Service {
	limiter := ratelimit.NewTokenBucketLimiter(jujuratelimit.NewBucketWithRate(100, 100))
	var createOrderEndpoint endpoint.Endpoint
	var getOrdersEndpoint endpoint.Endpoint
	{
		createOrderEndpoint = grpctransport.NewClient(
			conn,
			"pb.OrderRpcService",
			"CreateOrder",
			encodeGRPCCreateOrderRequest,
			decodeGRPCCreateOrderResponse,
			pb.CreatedOrderResponse{},
			grpctransport.ClientBefore(opentracing.ContextToGRPC(tracer, logger)),
		).Endpoint()
		createOrderEndpoint = opentracing.TraceClient(tracer, "CreateOrder")(createOrderEndpoint)
		createOrderEndpoint = limiter(createOrderEndpoint)
		createOrderEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "CreateOrder",
			Timeout: 30 * time.Second,
		}))(createOrderEndpoint)

		getOrdersEndpoint = grpctransport.NewClient(
			conn,
			"pb.OrderRpcService",
			"GetOrders",
			encodeGRPCGetOrdersRequest,
			decodeGRPCGetOrdersResponse,
			pb.GetOrdersResponse{},
			grpctransport.ClientBefore(opentracing.ContextToGRPC(tracer, logger)),
		).Endpoint()
		getOrdersEndpoint = opentracing.TraceClient(tracer, "GetOrders")(getOrdersEndpoint)
		getOrdersEndpoint = limiter(getOrdersEndpoint)
		getOrdersEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "GetOrders",
			Timeout: 30 * time.Second,
		}))(getOrdersEndpoint)
	}
	return o_endpoint.Set{
		CreateOrderEndpoint: createOrderEndpoint,
		GetOrdersEndpoint:   getOrdersEndpoint,
	}
}

func encodeGRPCCreateOrderRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(m_order.CreateOrderRequest)
	return &pb.CreateOrderRequest{
		Amount: float32(req.Amount),
	}, nil
}

func decodeGRPCCreateOrderResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.CreatedOrderResponse)
	return m_order.CreatedOrderResponse{
		ID:  reply.Id,
		Err: str2err(reply.Err)}, nil
}

// getOrders encode/decode func

func encodeGRPCGetOrdersRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(m_order.GetOrdersRequest)
	return &pb.GetOrdersRequest{
		Userid: req.UserID,
	}, nil
}

func decodeGRPCGetOrdersResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*pb.GetOrdersResponse)
	return m_order.GetOrdersResponse{
		Err: str2err(reply.Err)}, nil
}

func str2err(s string) error {
	if s == "" {
		return nil
	}
	return errors.New(s)
}

func err2str(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
