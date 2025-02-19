package proto

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

// extends proto.Message
type ReqGrpcTestMessage struct {
	// int32 does not implement Marshal
	Foo int32 `protobuf:"varint,1,opt,name=foo,proto3" json:"foo,omitempty"`
}

func (m *ReqGrpcTestMessage) Reset()         { *m = ReqGrpcTestMessage{} }
func (m *ReqGrpcTestMessage) String() string { return proto.CompactTextString(m) }
func (*ReqGrpcTestMessage) ProtoMessage()    {}

// extends proto.Message

type ResGrpcTestMessage struct {
	Bar int32 `protobuf:"varint,1,opt,name=bar,proto3" json:"bar,omitempty"`
}

func (m *ResGrpcTestMessage) Reset()         { *m = ResGrpcTestMessage{} }
func (m *ResGrpcTestMessage) String() string { return proto.CompactTextString(m) }
func (*ResGrpcTestMessage) ProtoMessage()    {}

type FooClient interface {
	TestFoo(ctx context.Context, in *ReqGrpcTestMessage, opts ...grpc.CallOption) (*ResGrpcTestMessage, error)
}

type FooClientImpl struct {
	cc *grpc.ClientConn
}

func NewGrpcTestClient(cc grpc.ClientConnInterface) FooClient {
	v := FooClientImpl{cc: cc.(*grpc.ClientConn)}
	return &v
}

func (c *FooClientImpl) TestFoo(ctx context.Context, in *ReqGrpcTestMessage, opts ...grpc.CallOption) (*ResGrpcTestMessage, error) {
	out := new(ResGrpcTestMessage)
	err := c.cc.Invoke(ctx, "/proto.Foo/TestFoo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type FooServer interface {
	TestFoo(context.Context, *ReqGrpcTestMessage) (*ResGrpcTestMessage, error)
}

type FooServerImpl struct {
}

func (s *FooServerImpl) TestFoo(ctx context.Context, in *ReqGrpcTestMessage) (*ResGrpcTestMessage, error) {
	if in.Foo == 1 {
		return &ResGrpcTestMessage{Bar: in.Foo}, nil
	} else {
		return nil, fmt.Errorf("invalid foo")
	}
}

func testFooHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReqGrpcTestMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FooServer).TestFoo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Foo/TestFoo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FooServer).TestFoo(ctx, req.(*ReqGrpcTestMessage))
	}
	return interceptor(ctx, in, info, handler)
}

var fooServicedesc = grpc.ServiceDesc{
	ServiceName: "proto.Foo",
	HandlerType: (*FooServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "TestFoo",
			Handler:    testFooHandler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto.proto",
}

func RegisterFooServer(s *grpc.Server, srv FooServer) {
	s.RegisterService(&fooServicedesc, srv)
}
