module github.com/Sendiw14/Clusterx/apps/control-plane

go 1.27

require (
	github.com/Sendiw14/Clusterx/examples/basic-jobs v0.0.0-00010101000000-000000000000
	github.com/Sendiw14/Clusterx/packages/proto v0.0.0-00010101000000-000000000000
	github.com/Sendiw14/Clusterx/packages/sdk-go v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.13.0
	github.com/prometheus/client_golang v1.25.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/Sendiw14/Clusterx/packages/proto => ../../packages/proto

replace github.com/Sendiw14/Clusterx/examples/basic-jobs => ../../examples/basic-jobs

replace github.com/Sendiw14/Clusterx/packages/sdk-go => ../../packages/sdk-go
