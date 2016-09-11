package identify_test

import (
	"testing"
	"time"

	ic "github.com/ipfs/go-libp2p-crypto"
	peer "github.com/ipfs/go-libp2p-peer"
	host "github.com/libp2p/go-libp2p/p2p/host"
	identify "github.com/libp2p/go-libp2p/p2p/protocol/identify"
	testutil "github.com/libp2p/go-libp2p/p2p/test/util"

	ma "github.com/jbenet/go-multiaddr"
	context "golang.org/x/net/context"
)

func subtestIDService(t *testing.T, postDialWait time.Duration) {

	// the generated networks should have the id service wired in.
	ctx := context.Background()
	h1 := testutil.GenHostSwarm(t, ctx)
	h2 := testutil.GenHostSwarm(t, ctx)

	h1p := h1.ID()
	h2p := h2.ID()

	testKnowsAddrs(t, h1, h2p, []ma.Multiaddr{}) // nothing
	testKnowsAddrs(t, h2, h1p, []ma.Multiaddr{}) // nothing

	h2pi := h2.Peerstore().PeerInfo(h2p)
	if err := h1.Connect(ctx, h2pi); err != nil {
		t.Fatal(err)
	}

	// we need to wait here if Dial returns before ID service is finished.
	if postDialWait > 0 {
		<-time.After(postDialWait)
	}

	// the IDService should be opened automatically, by the network.
	// what we should see now is that both peers know about each others listen addresses.
	t.Log("test peer1 has peer2 addrs correctly")
	testKnowsAddrs(t, h1, h2p, h2.Peerstore().Addrs(h2p)) // has them
	testHasProtocolVersions(t, h1, h2p)
	testHasPublicKey(t, h1, h2p, h2.Peerstore().PubKey(h2p)) // h1 should have h2's public key

	// now, this wait we do have to do. it's the wait for the Listening side
	// to be done identifying the connection.
	c := h2.Network().ConnsToPeer(h1.ID())
	if len(c) < 1 {
		t.Fatal("should have connection by now at least.")
	}
	<-h2.IDService().IdentifyWait(c[0])

	addrs := h1.Peerstore().Addrs(h1p)
	addrs = append(addrs, c[0].RemoteMultiaddr())

	// and the protocol versions.
	t.Log("test peer2 has peer1 addrs correctly")
	testKnowsAddrs(t, h2, h1p, addrs) // has them
	testHasProtocolVersions(t, h2, h1p)
	testHasPublicKey(t, h2, h1p, h1.Peerstore().PubKey(h1p)) // h1 should have h2's public key
}

func testKnowsAddrs(t *testing.T, h host.Host, p peer.ID, expected []ma.Multiaddr) {
	actual := h.Peerstore().Addrs(p)

	if len(actual) != len(expected) {
		t.Errorf("expected: %s", expected)
		t.Errorf("actual: %s", actual)
		t.Fatal("dont have the same addresses")
	}

	have := map[string]struct{}{}
	for _, addr := range actual {
		have[addr.String()] = struct{}{}
	}
	for _, addr := range expected {
		if _, found := have[addr.String()]; !found {
			t.Errorf("%s did not have addr for %s: %s", h.ID(), p, addr)
			// panic("ahhhhhhh")
		}
	}
}

func testHasProtocolVersions(t *testing.T, h host.Host, p peer.ID) {
	v, err := h.Peerstore().Get(p, "ProtocolVersion")
	if v == nil {
		t.Error("no protocol version")
		return
	}
	if v.(string) != identify.LibP2PVersion {
		t.Error("protocol mismatch", err)
	}
	v, err = h.Peerstore().Get(p, "AgentVersion")
	if v.(string) != identify.ClientVersion {
		t.Error("agent version mismatch", err)
	}
}

func testHasPublicKey(t *testing.T, h host.Host, p peer.ID, shouldBe ic.PubKey) {
	k := h.Peerstore().PubKey(p)
	if k == nil {
		t.Error("no public key")
		return
	}
	if !k.Equals(shouldBe) {
		t.Error("key mismatch")
		return
	}

	p2, err := peer.IDFromPublicKey(k)
	if err != nil {
		t.Error("could not make key")
	} else if p != p2 {
		t.Error("key does not match peerid")
	}
}

// TestIDServiceWait gives the ID service 100ms to finish after dialing
// this is becasue it used to be concurrent. Now, Dial wait till the
// id service is done.
func TestIDServiceWait(t *testing.T) {
	N := 3
	for i := 0; i < N; i++ {
		subtestIDService(t, 100*time.Millisecond)
	}
}

func TestIDServiceNoWait(t *testing.T) {
	N := 3
	for i := 0; i < N; i++ {
		subtestIDService(t, 0)
	}
}

func TestProtoMatching(t *testing.T) {
	tcp1, _ := ma.NewMultiaddr("/ip4/1.2.3.4/tcp/1234")
	tcp2, _ := ma.NewMultiaddr("/ip4/1.2.3.4/tcp/2345")
	tcp3, _ := ma.NewMultiaddr("/ip4/1.2.3.4/tcp/4567")
	utp, _ := ma.NewMultiaddr("/ip4/1.2.3.4/udp/1234/utp")

	if !identify.HasConsistentTransport(tcp1, []ma.Multiaddr{tcp2, tcp3, utp}) {
		t.Fatal("expected match")
	}

	if identify.HasConsistentTransport(utp, []ma.Multiaddr{tcp2, tcp3}) {
		t.Fatal("expected mismatch")
	}
}
