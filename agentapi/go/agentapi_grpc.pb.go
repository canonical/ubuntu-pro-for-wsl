// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.24.3
// source: agentapi.proto

package agentapi

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	UI_ApplyProToken_FullMethodName        = "/agentapi.UI/ApplyProToken"
	UI_ApplyLandscapeConfig_FullMethodName = "/agentapi.UI/ApplyLandscapeConfig"
	UI_Ping_FullMethodName                 = "/agentapi.UI/Ping"
	UI_GetConfigSources_FullMethodName     = "/agentapi.UI/GetConfigSources"
	UI_NotifyPurchase_FullMethodName       = "/agentapi.UI/NotifyPurchase"
)

// UIClient is the client API for UI service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type UIClient interface {
	ApplyProToken(ctx context.Context, in *ProAttachInfo, opts ...grpc.CallOption) (*SubscriptionInfo, error)
	ApplyLandscapeConfig(ctx context.Context, in *LandscapeConfig, opts ...grpc.CallOption) (*LandscapeSource, error)
	Ping(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error)
	GetConfigSources(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*ConfigSources, error)
	NotifyPurchase(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*SubscriptionInfo, error)
}

type uIClient struct {
	cc grpc.ClientConnInterface
}

func NewUIClient(cc grpc.ClientConnInterface) UIClient {
	return &uIClient{cc}
}

func (c *uIClient) ApplyProToken(ctx context.Context, in *ProAttachInfo, opts ...grpc.CallOption) (*SubscriptionInfo, error) {
	out := new(SubscriptionInfo)
	err := c.cc.Invoke(ctx, UI_ApplyProToken_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *uIClient) ApplyLandscapeConfig(ctx context.Context, in *LandscapeConfig, opts ...grpc.CallOption) (*LandscapeSource, error) {
	out := new(LandscapeSource)
	err := c.cc.Invoke(ctx, UI_ApplyLandscapeConfig_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *uIClient) Ping(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, UI_Ping_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *uIClient) GetConfigSources(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*ConfigSources, error) {
	out := new(ConfigSources)
	err := c.cc.Invoke(ctx, UI_GetConfigSources_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *uIClient) NotifyPurchase(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*SubscriptionInfo, error) {
	out := new(SubscriptionInfo)
	err := c.cc.Invoke(ctx, UI_NotifyPurchase_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UIServer is the server API for UI service.
// All implementations must embed UnimplementedUIServer
// for forward compatibility
type UIServer interface {
	ApplyProToken(context.Context, *ProAttachInfo) (*SubscriptionInfo, error)
	ApplyLandscapeConfig(context.Context, *LandscapeConfig) (*LandscapeSource, error)
	Ping(context.Context, *Empty) (*Empty, error)
	GetConfigSources(context.Context, *Empty) (*ConfigSources, error)
	NotifyPurchase(context.Context, *Empty) (*SubscriptionInfo, error)
	mustEmbedUnimplementedUIServer()
}

// UnimplementedUIServer must be embedded to have forward compatible implementations.
type UnimplementedUIServer struct {
}

func (UnimplementedUIServer) ApplyProToken(context.Context, *ProAttachInfo) (*SubscriptionInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ApplyProToken not implemented")
}
func (UnimplementedUIServer) ApplyLandscapeConfig(context.Context, *LandscapeConfig) (*LandscapeSource, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ApplyLandscapeConfig not implemented")
}
func (UnimplementedUIServer) Ping(context.Context, *Empty) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedUIServer) GetConfigSources(context.Context, *Empty) (*ConfigSources, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetConfigSources not implemented")
}
func (UnimplementedUIServer) NotifyPurchase(context.Context, *Empty) (*SubscriptionInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NotifyPurchase not implemented")
}
func (UnimplementedUIServer) mustEmbedUnimplementedUIServer() {}

// UnsafeUIServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to UIServer will
// result in compilation errors.
type UnsafeUIServer interface {
	mustEmbedUnimplementedUIServer()
}

func RegisterUIServer(s grpc.ServiceRegistrar, srv UIServer) {
	s.RegisterService(&UI_ServiceDesc, srv)
}

func _UI_ApplyProToken_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProAttachInfo)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UIServer).ApplyProToken(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UI_ApplyProToken_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UIServer).ApplyProToken(ctx, req.(*ProAttachInfo))
	}
	return interceptor(ctx, in, info, handler)
}

func _UI_ApplyLandscapeConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LandscapeConfig)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UIServer).ApplyLandscapeConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UI_ApplyLandscapeConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UIServer).ApplyLandscapeConfig(ctx, req.(*LandscapeConfig))
	}
	return interceptor(ctx, in, info, handler)
}

func _UI_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UIServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UI_Ping_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UIServer).Ping(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _UI_GetConfigSources_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UIServer).GetConfigSources(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UI_GetConfigSources_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UIServer).GetConfigSources(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _UI_NotifyPurchase_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UIServer).NotifyPurchase(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UI_NotifyPurchase_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UIServer).NotifyPurchase(ctx, req.(*Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// UI_ServiceDesc is the grpc.ServiceDesc for UI service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var UI_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "agentapi.UI",
	HandlerType: (*UIServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ApplyProToken",
			Handler:    _UI_ApplyProToken_Handler,
		},
		{
			MethodName: "ApplyLandscapeConfig",
			Handler:    _UI_ApplyLandscapeConfig_Handler,
		},
		{
			MethodName: "Ping",
			Handler:    _UI_Ping_Handler,
		},
		{
			MethodName: "GetConfigSources",
			Handler:    _UI_GetConfigSources_Handler,
		},
		{
			MethodName: "NotifyPurchase",
			Handler:    _UI_NotifyPurchase_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "agentapi.proto",
}

const (
	WSLInstance_Connected_FullMethodName               = "/agentapi.WSLInstance/Connected"
	WSLInstance_ProAttachmentCommands_FullMethodName   = "/agentapi.WSLInstance/ProAttachmentCommands"
	WSLInstance_LandscapeConfigCommands_FullMethodName = "/agentapi.WSLInstance/LandscapeConfigCommands"
)

// WSLInstanceClient is the client API for WSLInstance service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type WSLInstanceClient interface {
	Connected(ctx context.Context, opts ...grpc.CallOption) (WSLInstance_ConnectedClient, error)
	// Reverse unary calls
	ProAttachmentCommands(ctx context.Context, opts ...grpc.CallOption) (WSLInstance_ProAttachmentCommandsClient, error)
	LandscapeConfigCommands(ctx context.Context, opts ...grpc.CallOption) (WSLInstance_LandscapeConfigCommandsClient, error)
}

type wSLInstanceClient struct {
	cc grpc.ClientConnInterface
}

func NewWSLInstanceClient(cc grpc.ClientConnInterface) WSLInstanceClient {
	return &wSLInstanceClient{cc}
}

func (c *wSLInstanceClient) Connected(ctx context.Context, opts ...grpc.CallOption) (WSLInstance_ConnectedClient, error) {
	stream, err := c.cc.NewStream(ctx, &WSLInstance_ServiceDesc.Streams[0], WSLInstance_Connected_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &wSLInstanceConnectedClient{stream}
	return x, nil
}

type WSLInstance_ConnectedClient interface {
	Send(*DistroInfo) error
	CloseAndRecv() (*Empty, error)
	grpc.ClientStream
}

type wSLInstanceConnectedClient struct {
	grpc.ClientStream
}

func (x *wSLInstanceConnectedClient) Send(m *DistroInfo) error {
	return x.ClientStream.SendMsg(m)
}

func (x *wSLInstanceConnectedClient) CloseAndRecv() (*Empty, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *wSLInstanceClient) ProAttachmentCommands(ctx context.Context, opts ...grpc.CallOption) (WSLInstance_ProAttachmentCommandsClient, error) {
	stream, err := c.cc.NewStream(ctx, &WSLInstance_ServiceDesc.Streams[1], WSLInstance_ProAttachmentCommands_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &wSLInstanceProAttachmentCommandsClient{stream}
	return x, nil
}

type WSLInstance_ProAttachmentCommandsClient interface {
	Send(*Result) error
	Recv() (*ProAttachCmd, error)
	grpc.ClientStream
}

type wSLInstanceProAttachmentCommandsClient struct {
	grpc.ClientStream
}

func (x *wSLInstanceProAttachmentCommandsClient) Send(m *Result) error {
	return x.ClientStream.SendMsg(m)
}

func (x *wSLInstanceProAttachmentCommandsClient) Recv() (*ProAttachCmd, error) {
	m := new(ProAttachCmd)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *wSLInstanceClient) LandscapeConfigCommands(ctx context.Context, opts ...grpc.CallOption) (WSLInstance_LandscapeConfigCommandsClient, error) {
	stream, err := c.cc.NewStream(ctx, &WSLInstance_ServiceDesc.Streams[2], WSLInstance_LandscapeConfigCommands_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &wSLInstanceLandscapeConfigCommandsClient{stream}
	return x, nil
}

type WSLInstance_LandscapeConfigCommandsClient interface {
	Send(*Result) error
	Recv() (*LandscapeConfigCmd, error)
	grpc.ClientStream
}

type wSLInstanceLandscapeConfigCommandsClient struct {
	grpc.ClientStream
}

func (x *wSLInstanceLandscapeConfigCommandsClient) Send(m *Result) error {
	return x.ClientStream.SendMsg(m)
}

func (x *wSLInstanceLandscapeConfigCommandsClient) Recv() (*LandscapeConfigCmd, error) {
	m := new(LandscapeConfigCmd)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// WSLInstanceServer is the server API for WSLInstance service.
// All implementations must embed UnimplementedWSLInstanceServer
// for forward compatibility
type WSLInstanceServer interface {
	Connected(WSLInstance_ConnectedServer) error
	// Reverse unary calls
	ProAttachmentCommands(WSLInstance_ProAttachmentCommandsServer) error
	LandscapeConfigCommands(WSLInstance_LandscapeConfigCommandsServer) error
	mustEmbedUnimplementedWSLInstanceServer()
}

// UnimplementedWSLInstanceServer must be embedded to have forward compatible implementations.
type UnimplementedWSLInstanceServer struct {
}

func (UnimplementedWSLInstanceServer) Connected(WSLInstance_ConnectedServer) error {
	return status.Errorf(codes.Unimplemented, "method Connected not implemented")
}
func (UnimplementedWSLInstanceServer) ProAttachmentCommands(WSLInstance_ProAttachmentCommandsServer) error {
	return status.Errorf(codes.Unimplemented, "method ProAttachmentCommands not implemented")
}
func (UnimplementedWSLInstanceServer) LandscapeConfigCommands(WSLInstance_LandscapeConfigCommandsServer) error {
	return status.Errorf(codes.Unimplemented, "method LandscapeConfigCommands not implemented")
}
func (UnimplementedWSLInstanceServer) mustEmbedUnimplementedWSLInstanceServer() {}

// UnsafeWSLInstanceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to WSLInstanceServer will
// result in compilation errors.
type UnsafeWSLInstanceServer interface {
	mustEmbedUnimplementedWSLInstanceServer()
}

func RegisterWSLInstanceServer(s grpc.ServiceRegistrar, srv WSLInstanceServer) {
	s.RegisterService(&WSLInstance_ServiceDesc, srv)
}

func _WSLInstance_Connected_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(WSLInstanceServer).Connected(&wSLInstanceConnectedServer{stream})
}

type WSLInstance_ConnectedServer interface {
	SendAndClose(*Empty) error
	Recv() (*DistroInfo, error)
	grpc.ServerStream
}

type wSLInstanceConnectedServer struct {
	grpc.ServerStream
}

func (x *wSLInstanceConnectedServer) SendAndClose(m *Empty) error {
	return x.ServerStream.SendMsg(m)
}

func (x *wSLInstanceConnectedServer) Recv() (*DistroInfo, error) {
	m := new(DistroInfo)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _WSLInstance_ProAttachmentCommands_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(WSLInstanceServer).ProAttachmentCommands(&wSLInstanceProAttachmentCommandsServer{stream})
}

type WSLInstance_ProAttachmentCommandsServer interface {
	Send(*ProAttachCmd) error
	Recv() (*Result, error)
	grpc.ServerStream
}

type wSLInstanceProAttachmentCommandsServer struct {
	grpc.ServerStream
}

func (x *wSLInstanceProAttachmentCommandsServer) Send(m *ProAttachCmd) error {
	return x.ServerStream.SendMsg(m)
}

func (x *wSLInstanceProAttachmentCommandsServer) Recv() (*Result, error) {
	m := new(Result)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _WSLInstance_LandscapeConfigCommands_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(WSLInstanceServer).LandscapeConfigCommands(&wSLInstanceLandscapeConfigCommandsServer{stream})
}

type WSLInstance_LandscapeConfigCommandsServer interface {
	Send(*LandscapeConfigCmd) error
	Recv() (*Result, error)
	grpc.ServerStream
}

type wSLInstanceLandscapeConfigCommandsServer struct {
	grpc.ServerStream
}

func (x *wSLInstanceLandscapeConfigCommandsServer) Send(m *LandscapeConfigCmd) error {
	return x.ServerStream.SendMsg(m)
}

func (x *wSLInstanceLandscapeConfigCommandsServer) Recv() (*Result, error) {
	m := new(Result)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// WSLInstance_ServiceDesc is the grpc.ServiceDesc for WSLInstance service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var WSLInstance_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "agentapi.WSLInstance",
	HandlerType: (*WSLInstanceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Connected",
			Handler:       _WSLInstance_Connected_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "ProAttachmentCommands",
			Handler:       _WSLInstance_ProAttachmentCommands_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "LandscapeConfigCommands",
			Handler:       _WSLInstance_LandscapeConfigCommands_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "agentapi.proto",
}
