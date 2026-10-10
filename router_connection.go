package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
)

const tcpNetworkName = "tcp"

// CommandResult represents the outcome of a command executed over SSH.
type CommandResult struct {
	StandardOutput string
	StandardError  string
	ExitStatus     int
}

// CommandExecutor executes a shell command on the router and returns its result.
type CommandExecutor interface {
	ExecuteCommand(command string) (CommandResult, error)
}

// RouterConnection manages a reusable SSH connection to the OpenWrt router.
type RouterConnection struct {
	configuration  ServerConfiguration
	connectionLock sync.Mutex
	sshClient      *ssh.Client
}

// NewRouterConnection creates a RouterConnection for the provided configuration.
func NewRouterConnection(configuration ServerConfiguration) *RouterConnection {
	routerConnection := &RouterConnection{
		configuration: configuration,
	}
	return routerConnection
}

// ExecuteCommand runs a single shell command, reconnecting once if the cached
// SSH connection has become unusable.
func (routerConnection *RouterConnection) ExecuteCommand(command string) (CommandResult, error) {
	session, sessionError := routerConnection.createSession()
	if sessionError == nil {
		return runCommandInSession(session, command)
	}

	routerConnection.invalidateConnection()

	retriedSession, retriedSessionError := routerConnection.createSession()
	if retriedSessionError != nil {
		emptyResult := CommandResult{}
		return emptyResult, retriedSessionError
	}

	return runCommandInSession(retriedSession, command)
}

func runCommandInSession(session *ssh.Session, command string) (CommandResult, error) {
	defer session.Close()

	var standardOutputBuffer bytes.Buffer
	var standardErrorBuffer bytes.Buffer

	session.Stdout = &standardOutputBuffer
	session.Stderr = &standardErrorBuffer

	runError := session.Run(command)

	commandResult := CommandResult{
		StandardOutput: standardOutputBuffer.String(),
		StandardError:  standardErrorBuffer.String(),
		ExitStatus:     0,
	}

	if runError == nil {
		return commandResult, nil
	}

	var exitError *ssh.ExitError
	exitErrorDetected := errors.As(runError, &exitError)
	if exitErrorDetected {
		commandResult.ExitStatus = exitError.ExitStatus()
	} else {
		commandResult.ExitStatus = -1
	}

	return commandResult, runError
}

func (routerConnection *RouterConnection) createSession() (*ssh.Session, error) {
	sshClient, connectionError := routerConnection.ensureConnection()
	if connectionError != nil {
		return nil, connectionError
	}

	session, sessionError := sshClient.NewSession()
	if sessionError != nil {
		return nil, sessionError
	}

	return session, nil
}

func (routerConnection *RouterConnection) ensureConnection() (*ssh.Client, error) {
	routerConnection.connectionLock.Lock()
	defer routerConnection.connectionLock.Unlock()

	if routerConnection.sshClient != nil {
		return routerConnection.sshClient, nil
	}

	authenticationMethods, authenticationError := buildAuthenticationMethods(routerConnection.configuration)
	if authenticationError != nil {
		return nil, authenticationError
	}

	hostKeyCallback, hostKeyError := buildHostKeyCallback(routerConnection.configuration)
	if hostKeyError != nil {
		return nil, hostKeyError
	}

	clientConfiguration := &ssh.ClientConfig{
		User:            routerConnection.configuration.RouterUsername,
		Auth:            authenticationMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         routerConnection.configuration.RouterConnectionTimeout,
	}

	sshClient, dialError := ssh.Dial(tcpNetworkName, routerConnection.configuration.RouterAddress, clientConfiguration)
	if dialError != nil {
		return nil, dialError
	}

	routerConnection.sshClient = sshClient
	return sshClient, nil
}

func (routerConnection *RouterConnection) invalidateConnection() {
	routerConnection.connectionLock.Lock()
	defer routerConnection.connectionLock.Unlock()

	if routerConnection.sshClient != nil {
		routerConnection.sshClient.Close()
		routerConnection.sshClient = nil
	}
}

func buildAuthenticationMethods(configuration ServerConfiguration) ([]ssh.AuthMethod, error) {
	authenticationMethods := make([]ssh.AuthMethod, 0)

	if configuration.RouterPassword != "" {
		passwordAuthenticationMethod := ssh.Password(configuration.RouterPassword)
		authenticationMethods = append(authenticationMethods, passwordAuthenticationMethod)
	}

	privateKeyBytes := configuration.RouterPrivateKey
	if len(privateKeyBytes) == 0 {
		if configuration.RouterPrivateKeyPath != "" {
			readBytes, readError := os.ReadFile(configuration.RouterPrivateKeyPath)
			if readError != nil {
				return nil, readError
			}
			privateKeyBytes = readBytes
		}
	}

	if len(privateKeyBytes) > 0 {
		privateKeySigner, signerError := parsePrivateKeySigner(privateKeyBytes, configuration.RouterPrivateKeyPassphrase)
		if signerError != nil {
			return nil, signerError
		}

		publicKeyAuthenticationMethod := ssh.PublicKeys(privateKeySigner)
		authenticationMethods = append(authenticationMethods, publicKeyAuthenticationMethod)
	}

	if len(authenticationMethods) == 0 {
		missingAuthenticationError := errors.New("no SSH authentication method is configured; set ROUTER_PASSWORD or provide a private key")
		return nil, missingAuthenticationError
	}

	return authenticationMethods, nil
}

func parsePrivateKeySigner(privateKeyBytes []byte, privateKeyPassphrase string) (ssh.Signer, error) {
	if privateKeyPassphrase != "" {
		privateKeySigner, parseError := ssh.ParsePrivateKeyWithPassphrase(privateKeyBytes, []byte(privateKeyPassphrase))
		if parseError != nil {
			return nil, parseError
		}
		return privateKeySigner, nil
	}

	privateKeySigner, parseError := ssh.ParsePrivateKey(privateKeyBytes)
	if parseError != nil {
		return nil, parseError
	}
	return privateKeySigner, nil
}

func buildHostKeyCallback(configuration ServerConfiguration) (ssh.HostKeyCallback, error) {
	if configuration.RouterHostKeyFingerprint == "" {
		insecureCallback := ssh.InsecureIgnoreHostKey()
		return insecureCallback, nil
	}

	fingerprintVerifier := fingerprintHostKeyVerifier{
		expectedFingerprint: configuration.RouterHostKeyFingerprint,
	}
	verificationCallback := fingerprintVerifier.verify
	return verificationCallback, nil
}

type fingerprintHostKeyVerifier struct {
	expectedFingerprint string
}

func (verifier fingerprintHostKeyVerifier) verify(hostname string, remoteAddress net.Addr, hostKey ssh.PublicKey) error {
	actualFingerprint := ssh.FingerprintSHA256(hostKey)
	if actualFingerprint == verifier.expectedFingerprint {
		return nil
	}

	errorMessage := fmt.Sprintf(
		"SSH host key verification failed for %s: expected fingerprint %s but received %s",
		hostname,
		verifier.expectedFingerprint,
		actualFingerprint,
	)
	verificationError := errors.New(errorMessage)
	return verificationError
}
