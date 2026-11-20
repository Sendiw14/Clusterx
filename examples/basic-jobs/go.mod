module github.com/Sendiw14/Clusterx/examples/basic-jobs

go 1.27

require github.com/Sendiw14/Clusterx/packages/sdk-go v0.0.0-00010101000000-000000000000

require (
	github.com/Sendiw14/Clusterx/packages/proto v0.0.0-00010101000000-000000000000 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1 // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace github.com/Sendiw14/Clusterx/packages/sdk-go => ../../packages/sdk-go

replace github.com/Sendiw14/Clusterx/packages/proto => ../../packages/proto
