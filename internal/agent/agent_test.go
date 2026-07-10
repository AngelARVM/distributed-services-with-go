package agent_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/travisjeffery/go-dynaport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	api "github.com/angelarvm/prolog/api/v1"
	"github.com/angelarvm/prolog/internal/agent"
	config "github.com/angelarvm/prolog/internal/config"
)

func TestAgent(t *testing.T) {
	tlsDir, err := ioutil.TempDir("", "agent-test-tls")
	require.NoError(t, err)
	defer os.RemoveAll(tlsDir)

	tlsFiles := generateTLSFiles(t, tlsDir, "127.0.0.1")

	serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      tlsFiles.serverCertFile,
		KeyFile:       tlsFiles.serverKeyFile,
		CAFile:        tlsFiles.caFile,
		Server:        true,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	peerTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile:      tlsFiles.rootClientCertFile,
		KeyFile:       tlsFiles.rootClientKeyFile,
		CAFile:        tlsFiles.caFile,
		Server:        false,
		ServerAddress: "127.0.0.1",
	})
	require.NoError(t, err)

	var agents []*agent.Agent
	for i := range 3 {
		ports := dynaport.Get(2)
		bindAddr := fmt.Sprintf("%s:%d", "127.0.0.1", ports[0])
		rpcPort := ports[1]

		dataDir, err := ioutil.TempDir("", "agent-test-log")
		require.NoError(t, err)

		var startJoinAddrs []string

		if i != 0 {
			startJoinAddrs = append(startJoinAddrs, agents[0].Config.BindAddr)
		}

		agent, err := agent.New(agent.Config{
			NodeName:        fmt.Sprintf("%d", i),
			StartJoinAddrs:  startJoinAddrs,
			BindAddr:        bindAddr,
			RPCPort:         rpcPort,
			DataDir:         dataDir,
			ACLModelFile:    testConfigFile("model.conf"),
			ACLPolicyFile:   testConfigFile("policy.csv"),
			ServerTLSConfig: serverTLSConfig,
			PeerTLSConfig:   peerTLSConfig,
		})

		require.NoError(t, err)

		agents = append(agents, agent)
	}

	defer func() {
		for _, agent := range agents {
			err := agent.Shutdown()
			require.NoError(t, err)
			require.NoError(t, os.RemoveAll(agent.Config.DataDir))
		}
	}()
	time.Sleep(3 * time.Second)

	leaderClient := client(t, agents[0], peerTLSConfig)
	produceResponse, err := leaderClient.Produce(
		context.Background(),
		&api.ProduceRequest{
			Record: &api.Record{
				Value: []byte("foo"),
			},
		},
	)
	require.NoError(t, err)

	consumeResponse, err := leaderClient.Consume(
		context.Background(),
		&api.ConsumeRequest{
			Offset: produceResponse.Offset,
		},
	)
	require.NoError(t, err)
	require.Equal(t, consumeResponse.Record.Value, []byte("foo"))

	time.Sleep(3 * time.Second)
	followerClient := client(t, agents[1], peerTLSConfig)
	consumeResponse, err = followerClient.Consume(
		context.Background(),
		&api.ConsumeRequest{
			Offset: produceResponse.Offset,
		},
	)
	require.Equal(t, consumeResponse.Record.Value, []byte("foo"))
}

func client(t *testing.T, agent *agent.Agent, tlsConfig *tls.Config) api.LogClient {
	tlsCreds := credentials.NewTLS(tlsConfig)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(tlsCreds)}
	rpcAddr, err := agent.Config.RPCAddr()
	require.NoError(t, err)
	conn, err := grpc.Dial(fmt.Sprintf("%s", rpcAddr), opts...)
	require.NoError(t, err)
	client := api.NewLogClient(conn)

	return client
}

type tlsFiles struct {
	caFile               string
	serverCertFile       string
	serverKeyFile        string
	rootClientCertFile   string
	rootClientKeyFile    string
	nobodyClientCertFile string
	nobodyClientKeyFile  string
}

func generateTLSFiles(t *testing.T, dir, host string) tlsFiles {
	t.Helper()

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"prolog test ca"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"prolog test server"},
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP(host)},
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	require.NoError(t, err)

	files := tlsFiles{
		caFile:               filepath.Join(dir, "ca.pem"),
		serverCertFile:       filepath.Join(dir, "server.pem"),
		serverKeyFile:        filepath.Join(dir, "server-key.pem"),
		rootClientCertFile:   filepath.Join(dir, "root-client.pem"),
		rootClientKeyFile:    filepath.Join(dir, "root-client-key.pem"),
		nobodyClientCertFile: filepath.Join(dir, "nobody-client.pem"),
		nobodyClientKeyFile:  filepath.Join(dir, "nobody-client-key.pem"),
	}

	require.NoError(t, os.WriteFile(files.caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0o600))
	require.NoError(t, os.WriteFile(files.serverCertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), 0o600))
	require.NoError(t, os.WriteFile(files.serverKeyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)}), 0o600))
	writeClientCert(t, caKey, caCert, files.rootClientCertFile, files.rootClientKeyFile, "root", 3)
	writeClientCert(t, caKey, caCert, files.nobodyClientCertFile, files.nobodyClientKeyFile, "nobody", 4)

	return files
}

func writeClientCert(
	t *testing.T,
	caKey *rsa.PrivateKey,
	caCert *x509.Certificate,
	certFile string,
	keyFile string,
	commonName string,
	serial int64,
) {
	t.Helper()

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"prolog test client"},
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}), 0o600))
}

func testConfigFile(name string) string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("could not resolve test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "test", name)
}
