package p2p

import (
	"math/rand"
	"net"

	"github.com/wangxinyu2018/mass-core/config"
	"github.com/wangxinyu2018/mass-core/p2p/connection"
	crypto "github.com/massnetorg/tendermint/go-crypto"
	cmn "github.com/massnetorg/tendermint/tmlibs/common"
)

//PanicOnAddPeerErr add peer error
var PanicOnAddPeerErr = false

func CreateRandomPeer(outbound bool) *Peer {
	_, netAddr := CreateRoutableAddr()
	p := &Peer{
		peerConn: &peerConn{
			outbound: outbound,
		},
		NodeInfo: &NodeInfo{
			ListenAddr: netAddr.DialString(),
		},
		mconn: &connection.MConnection{},
	}
	return p
}

func CreateRoutableAddr() (addr string, netAddr *NetAddress) {
	for {
		var err error
		addr = cmn.Fmt("%X@%v.%v.%v.%v:46656", cmn.RandBytes(20), cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256, cmn.RandInt()%256)
		netAddr, err = NewNetAddressString(addr)
		if err != nil {
			panic(err)
		}
		if netAddr.Routable() {
			break
		}
	}
	return
}

// MakeConnectedSwitches switches connected via arbitrary net.Conn; useful for testing
// Returns n switches, connected according to the connect func.
// If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the ith switch should be initialized (ie. with what reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(cfg *config.Config, n int, initSwitch func(int, *Switch) *Switch, connect func([]*Switch, int, int)) []*Switch {
	var err error
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i], err = MakeSwitch(cfg, i, "testing", "123.123.123", initSwitch)
		if err != nil {
			panic(err)
		}
	}

	if err := startSwitches(switches); err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			connect(switches, i, j)
		}
	}

	return switches
}

// Connect2Switches will connect switches i and j via net.Pipe()
// Blocks until a conection is established.
// NOTE: caller ensures i and j are within bounds
func Connect2Switches(switches []*Switch, i, j int) {
	switchI := switches[i]
	switchJ := switches[j]
	c1, c2 := net.Pipe()
	doneCh := make(chan struct{})
	go func() {
		err := switchI.addPeerWithConnection(c1)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	go func() {
		err := switchJ.addPeerWithConnection(c2)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	<-doneCh
}

func startSwitches(switches []*Switch) error {
	for _, s := range switches {
		_, err := s.Start() // start switch and reactors
		if err != nil {
			return err
		}
	}
	return nil
}

func MakeSwitch(cfg *config.Config, i int, network, version string, initSwitch func(int, *Switch) *Switch) (*Switch, error) {
	privKey := crypto.GenPrivKeyEd25519()
	// new switch, add reactors
	// TODO: let the config be passed in?
	sw, err := NewSwitch(cfg)
	if err != nil {
		return nil, err
	}
	s := initSwitch(i, sw)
	s.SetNodeInfo(&NodeInfo{
		PubKey:     privKey.PubKey().Unwrap().(crypto.PubKeyEd25519),
		Moniker:    cmn.Fmt("switch%d", i),
		Network:    network,
		Version:    version,
		ListenAddr: cmn.Fmt("%v:%v", network, rand.Intn(64512)+1023),
	})
	s.SetNodePrivKey(privKey)
	return s, nil
}
