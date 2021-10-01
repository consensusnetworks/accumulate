package api

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AccumulateNetwork/accumulated/networks"

	cfg "github.com/AccumulateNetwork/accumulated/config"
	"github.com/AccumulateNetwork/accumulated/internal/abci"
	"github.com/AccumulateNetwork/accumulated/internal/chain"
	"github.com/AccumulateNetwork/accumulated/internal/node"
	"github.com/AccumulateNetwork/accumulated/types/state"
	tmcfg "github.com/tendermint/tendermint/config"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/privval"
)

func randomRouterPorts() *cfg.API {
	port, err := tmnet.GetFreePort()
	if err != nil {
		panic(err)
	}
	return &cfg.API{
		JSONListenAddress: fmt.Sprintf("localhost:%d", port),
		RESTListenAddress: fmt.Sprintf("localhost:%d", port+1),
	}
}

func initOptsForNetwork(t *testing.T, name string) node.InitOptions {
	t.Helper()

	network := networks.Networks[name]
	listenIP := make([]string, len(network.Nodes))
	remoteIP := make([]string, len(network.Nodes))
	config := make([]*cfg.Config, len(network.Nodes))

	for i, net := range network.Nodes {
		listenIP[i] = "tcp://0.0.0.0"
		remoteIP[i] = net.IP
		config[i] = new(cfg.Config)
		config[i].Accumulate.Type = network.Type
		config[i].Accumulate.Networks = []string{remoteIP[0]}

		switch net.Type {
		case cfg.Validator:
			config[i].Config = *tmcfg.DefaultValidatorConfig()
		case cfg.Follower:
			config[i].Config = *tmcfg.DefaultValidatorConfig()
		default:
			fmt.Fprintf(os.Stderr, "Error: hard-coded network has invalid node type: %q\n", net.Type)
			os.Exit(1)
		}
	}

	return node.InitOptions{
		ShardName: "accumulate.",
		ChainID:   network.Name,
		Port:      network.Port,
		Config:    config,
		RemoteIP:  remoteIP,
		ListenIP:  listenIP,
	}
}

func boostrapBVC(t *testing.T, workingdir string) {
	t.Helper()

	opts := initOptsForNetwork(t, "Badlands")
	opts.WorkDir = workingdir

	for _, config := range opts.Config {
		//[mempool]
		//	broadcast = true
		//	cache_size = 100000
		//	max_batch_bytes = 10485760
		//	max_tx_bytes = 1048576
		//	max_txs_bytes = 1073741824
		//	recheck = true
		//	size = 50000
		//	wal_dir = ""
		//

		// config.Mempool.KeepInvalidTxsInCache = false
		// config.Mempool.MaxTxsBytes = 1073741824
		config.Mempool.MaxBatchBytes = 1048576
		config.Mempool.CacheSize = 1048576
		config.Mempool.Size = 50000
	}

	err := node.Init(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func newBVC(t *testing.T, workingdir string) (*cfg.Config, *privval.FilePV, *node.Node) {
	t.Helper()

	cfg, err := cfg.Load(workingdir)
	if err != nil {
		panic(err)
	}

	dbPath := filepath.Join(cfg.RootDir, "valacc.db")
	bvcId := sha256.Sum256([]byte(cfg.Instrumentation.Namespace))
	sdb := new(state.StateDB)
	err = sdb.Open(dbPath, bvcId[:], false, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database %s: %v", dbPath, err)
		os.Exit(1)
	}

	// read private validator
	pv, err := privval.LoadFilePV(
		cfg.PrivValidator.KeyFile(),
		cfg.PrivValidator.StateFile(),
	)
	if err != nil {
		t.Fatal(err)
	}

	bvc := chain.NewBlockValidator()
	mgr, err := chain.NewManager(cfg, sdb, pv.Key.PrivKey.Bytes(), bvc)
	if err != nil {
		t.Fatal(err)
	}

	app, err := abci.NewAccumulator(sdb, pv.Key.PubKey.Address(), mgr)
	if err != nil {
		t.Fatal(err)
	}

	node, err := node.New(cfg, app)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, pv, node
}

func startBVC(t *testing.T, dir string) (*cfg.Config, *privval.FilePV, *node.Node) {
	t.Helper()

	//generate the config files needed to run a test BVC
	boostrapBVC(t, dir)

	///Build a BVC we'll use for our test
	cfg, pv, node := newBVC(t, filepath.Join(dir, "Node0"))
	err := node.Start()
	if err != nil {
		t.Fatal(err)
	}

	return cfg, pv, node
}
