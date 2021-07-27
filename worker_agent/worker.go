package Agent

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipns"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/multiformats/go-multihash"
	"modified-agent-ipfs/config"
	//"github.com/ipfs/go-ipns"

	"strconv"
	"time"

	"github.com/ipfs/go-path/resolver"

	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/libp2p/go-libp2p-core/peer"
	kad "github.com/peter-tmc/go-libp2p-kad-dht"
	"go.uber.org/zap"
)

type Agent struct {
	host    host.Host
	public  crypto.PubKey
	priv crypto.PrivKey
	logger  zap.Logger
	getKeys []string
	dht     *kad.IpfsDHT
	res		resolver.Resolver
	totalNoAgents int
	agentNo int
	keyIndexes []int
}

func NewAgent(ctx context.Context, cfg config.Configuration) *Agent {
	rawJSON := []byte(`{
		"level": "info",
		"encoding": "json",
		"outputPaths": ["./logs/log.txt"],
		"errorOutputPaths": ["./logs/log.txt"],
		"encoderConfig": {
		  "messageKey": "main",
		  "levelKey": "level",
		  "levelEncoder": "lowercase"
		}
	  }`)
	var zapCfg zap.Config
	if err := json.Unmarshal(rawJSON, &zapCfg); err != nil {
		panic(err)
	}
	logger, err := zapCfg.Build()
	if err != nil {
		panic(err)
	}

	w := &Agent{
		logger: *logger,
		totalNoAgents: cfg.Agent.NoAgents,
		agentNo: cfg.Agent.AgentN,
	}
	w.logger.Info("Creating key pair")
	priv, public, _ := crypto.GenerateKeyPair(crypto.RSA, 2048)
	w.logger.Info("Created key pair; public key with type " + public.Type().String() +
		"; private key with type " + priv.Type().String())
	opts := []libp2p.Option{libp2p.Identity(priv)}
	h, err := libp2p.New(ctx, opts...)
	if err != nil {
		panic(err)
	}
	w.host = h
	w.public = public
	w.priv=priv
	w.keyIndexes = keyIndexesToProvide(w.agentNo, w.totalNoAgents)
	/*dag.NewReadOnlyDagService()
	resolv := resolver.NewBasicResolver()*/
	dht, err := kad.New(ctx, h, kad.Mode(1), kad.BootstrapPeers(kad.GetDefaultBootstrapPeerAddrInfos()...))
	if err != nil {
		w.logger.Panic(err.Error())
	}
	w.dht = dht
	w.logger.Info("DHT created successfully with mode " + strconv.Itoa(int(w.dht.Mode())) +
		" ID in the DHT is " + w.dht.PeerID().String())
	w.logger.Info("DHT bootstrapping")
	for i, v := range kad.GetDefaultBootstrapPeerAddrInfos() {
		logger.Info("bootstrap peer " + strconv.Itoa(i) + " " + v.String())
	}

	err = w.dht.Bootstrap(ctx)
	logger.Info("sleeping for 30 seconds")
	time.Sleep(30000*time.Millisecond)
	logger.Info("DHT Mode " + strconv.Itoa(int(w.dht.Mode())))
	if err != nil {
		return nil
	}
	w.logger.Info("DHT bootstrap complete")
	w.dht.RoutingTable().Print()
	/*<-w.dht.RefreshRoutingTable()
	logger.Info("refresh finished")
	w.dht.RoutingTable().Print()*/
	w.logger.Info("Created agent at " + time.Now().UTC().String())
	return w
}

func (a *Agent) GetClosestPeers(ctx context.Context, key string) ([]peer.ID, error) {
	a.logger.Info("had " + strconv.Itoa(a.dht.RoutingTable().Size()) + " peers")
	t1 := time.Now()
	a.logger.Info("Getting closest peers to key " + key + "peer ID from key is " + peer.ID(key).Pretty())
	by, err := a.dht.GetClosestPeers(ctx, key)
	if err != nil {
		a.logger.Panic(err.Error())
		panic(err)
	}
	t2 := time.Now()
	a.logger.Info("Successful in getting closest peers to key " + key +
		"peer ID from key is " + peer.ID(key).Pretty() + " took " + (t2.Sub(t1).String()))
	for _, v := range by {
		a.logger.Info("Found peer " + v.String())
	}
	return by, nil
}

func (a *Agent) GetValue(ctx context.Context, key string, hash bool) ([]kad.RecvdVal, error) {
	/*if(strings.Contains(key, "ipfs") || strings.Contains(key, "ipns")) {
	}*/
	a.logger.Info("had " + strconv.Itoa(a.dht.RoutingTable().Size()) + " peers")

	a.logger.Info("Getting value associated with key " + key)
	k := key
	if hash {
		mh1, err := multihash.Encode([]byte(key), multihash.SHA2_256) //isto devolve um array de bytes que se pode dar cast para uma multihash
		if err != nil {
		a.logger.Panic(err.Error())
		return nil, err
	}
		k = string(mh1)
		a.logger.Info("Converted key " + key + " to " + k)
	}
	t1 := time.Now()
	by, err := a.dht.GetValues(ctx, k, 100)

	if err != nil {
		a.logger.Panic(err.Error())
		panic(err)
	}
	t2 := time.Now()
	a.logger.Info("Successful in getting value associated with key " + k + " took " +
		(t2.Sub(t1).String()) + " got " + strconv.Itoa(len(by)) + " values")
	for _, v := range by {
		a.logger.Info("Got value: " + string(v.Val) + " from peer " + string(v.From))
	}
	return by, nil
}
/*na funcao get de ficheiros no ipfs imprimir para o log os nos que passamos
ordenados temporalmente e adicionalmente fazer comparacoes para ter a certeza
que estao a seguir caminhos corretos, que estao a tentar sempre ir pelo melhor
caminho que tem ate ao momento, para isso vemos quais os nos que estao a
seguir e adicionalmente todos os que tem no momento para ter a certeza que
esta sempre a escolher os mais proximos*/

/*
	para mandar um find node
	err := dht.WriteMsg(remotePeerStream, pb.NewMessage(pb.Message_FIND_NODE, target, 0))
*/

func (a *Agent) PutValue(ctx context.Context, key string, val []byte) error {
	a.logger.Info("had " + strconv.Itoa(a.dht.RoutingTable().Size()) + " peers")
	v1, err := ipns.Create(a.priv, val, 0, time.Now().Add(1*time.Hour), 0)
	if err != nil {
		a.logger.Panic(err.Error())
		return err
	}
	v, err := v1.Marshal()
	if err != nil {
		a.logger.Panic(err.Error())
		return err
	}
	t1 := time.Now()
	a.logger.Info("Putting value " + v1.String() + " with key " + key)
	err = a.dht.PutValue(ctx, "/ipns/"+a.dht.PeerID().String(), v)
	if err != nil {
		a.logger.Panic(err.Error())
		return err
	}
	t2 := time.Now()
	a.logger.Info("Successful in putting value " + string(val) + " with key " + key +
		" took " + (t2.Sub(t1).String()))

	return nil
}

func (a *Agent) ProvideVal(ctx context.Context, key []byte, index int, id uuid.UUID) error {
	a.logger.Info("had " + strconv.Itoa(a.dht.RoutingTable().Size()) + " peers")
	//criar multi hash
	mh1, err := multihash.Encode(key, multihash.SHA2_256) //isto devolve um array de bytes que se pode dar cast para uma multihash
	if err != nil {
		a.logger.Panic(err.Error())
		return err
	}
	mh, err := multihash.Cast(mh1)
	if err != nil {
		a.logger.Panic(err.Error())
		return err
	}
	//criar Cid
	c:= cid.NewCidV0(mh)
	t1 := time.Now()
	a.logger.Info("Providing value with key " + string(key) + " of index " + strconv.Itoa(index) + " request ID is " +
		id.String() + " peer ID from key is " + peer.ID(key).Pretty() + " at time " + time.Now().UTC().String())
	//fazer o provide
	err = a.dht.ProvideMod(ctx, c, true, id)
	if err != nil {
		a.logger.Panic(err.Error())
		return err
	}
	t2 := time.Now()
	a.logger.Info("Successful in providing value with key " + string(key)+ " of index " + strconv.Itoa(index) +
		" request ID is " + id.String() + " peer ID from key is " + peer.ID(key).Pretty() +
		" took " + (t2.Sub(t1).String()) + " at time " + time.Now().UTC().String())

	return nil
}

func keyIndexesToProvide(agentNo, totalNoAgents int) []int {
	var res [256]int
	count := 0
	for i := agentNo; i < 256; i+=totalNoAgents {
		res[count] = i
		count++
	}
	return res[:count]
}

func (a *Agent) GetIndexes() []int {
	return a.keyIndexes
}

func (a *Agent) CheckValidKeyIndex(i int) bool {
	return ((i-a.agentNo)%a.totalNoAgents) == 0
}

func (a *Agent) FindProviders(ctx context.Context, key []byte, index int, uid uuid.UUID) (p []peer.AddrInfo, e error) {
	a.logger.Info("had " + strconv.Itoa(a.dht.RoutingTable().Size()) + " peers")
	//criar multi hash
	mh1, err := multihash.Encode(key, multihash.SHA2_256) //isto devolve um array de bytes que se pode dar cast para uma multihash
	if err != nil {
		a.logger.Panic(err.Error())
		return nil, err
	}
	mh, err := multihash.Cast(mh1)
	if err != nil {
		a.logger.Panic(err.Error())
		return nil, err
	}
	//criar Cid
	c:= cid.NewCidV0(mh)
	t1 := time.Now()
	a.logger.Info("Finding providers with key " + string(key) + " of index "+ strconv.Itoa(index) + " ID is " + uid.String() +
		" peer ID from key is " + peer.ID(key).Pretty() + " at time " + time.Now().UTC().String())
	//fazer o provide
	peers, err := a.dht.FindProvidersMod(ctx, c, uid) // TODO meter o uuid la pa dentro
	if err != nil {
		a.logger.Panic(err.Error())
		return nil, err
	}
	t2 := time.Now()
	a.logger.Info("Successful in finding providers with key " + string(key) + " of index "+ strconv.Itoa(index) +  " ID is " + uid.String() +
		" peer ID from key is " + peer.ID(key).Pretty() + " took " + (t2.Sub(t1).String()) +
		" at time " + time.Now().UTC().String())

	return peers, nil
}