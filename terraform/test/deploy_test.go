//go:build integration

// Package deploytest deploys each OpenTofu module in terraform/<cloud> against
// a local floci emulator, checks the emulator's own API for what the module
// should have created, then destroys it and checks that it is gone.
//
// Run with `make test-deploy`, which starts the emulators from
// terraform/test/docker-compose.yml first.
package deploytest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const repoURL = "https://github.com/herom-s/ft_hackthon.git"

var wantPorts = []string{"22", "80", "3222", "8443"}

// testPublicKey returns a freshly generated RSA public key in authorized_keys
// form. azurerm parses the key and Azure Linux VMs only accept RSA, so a
// placeholder string is not enough.
func testPublicKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
}

func endpoint(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return strings.TrimRight(v, "/")
	}
	return fallback
}

// stack is one copy of a deploy module plus the floci test files, staged in a
// temp dir so the real module directory never gets .terraform or state files.
type stack struct {
	t   *testing.T
	dir string
	env []string
}

func newStack(t *testing.T, cloud string, env map[string]string) *stack {
	t.Helper()
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Fatal("tofu not found in PATH: install OpenTofu to run the deploy tests")
	}
	dir := t.TempDir()
	copyFiles(t, filepath.Join("..", cloud), dir, "*.tf", ".terraform.lock.hcl")
	copyFiles(t, cloud, dir, "*.tf")

	s := &stack{t: t, dir: dir, env: append(os.Environ(), "TF_IN_AUTOMATION=1")}
	if cache := os.Getenv("TF_PLUGIN_CACHE_DIR"); cache != "" {
		os.MkdirAll(cache, 0o755)
	}
	for k, v := range env {
		s.env = append(s.env, k+"="+v)
	}
	return s
}

func copyFiles(t *testing.T, from, to string, patterns ...string) {
	t.Helper()
	for _, p := range patterns {
		matches, err := filepath.Glob(filepath.Join(from, p))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(to, filepath.Base(m)), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func (s *stack) tofu(args ...string) (string, error) {
	cmd := exec.Command("tofu", append(args, "-no-color")...)
	cmd.Dir = s.dir
	cmd.Env = s.env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (s *stack) mustTofu(args ...string) {
	s.t.Helper()
	if out, err := s.tofu(args...); err != nil {
		s.t.Fatalf("tofu %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// deploy runs init + apply and registers a destroy that runs even when a later
// assertion fails, so a broken test never leaves resources in the emulator.
func (s *stack) deploy() map[string]string {
	s.t.Helper()
	s.mustTofu("init", "-input=false")
	s.t.Cleanup(func() {
		if out, err := s.tofu("destroy", "-input=false", "-auto-approve"); err != nil {
			s.t.Errorf("cleanup destroy failed: %v\n%s", err, out)
		}
	})
	s.mustTofu("apply", "-input=false", "-auto-approve")

	out, err := s.tofu("output", "-json")
	if err != nil {
		s.t.Fatalf("tofu output: %v\n%s", err, out)
	}
	var raw map[string]struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		s.t.Fatalf("parse outputs: %v\n%s", err, out)
	}
	outputs := map[string]string{}
	for k, v := range raw {
		outputs[k] = v.Value
	}
	return outputs
}

func (s *stack) destroy() {
	s.t.Helper()
	s.mustTofu("destroy", "-input=false", "-auto-approve")
}

func checkOutputs(t *testing.T, outputs map[string]string, sshUser string) string {
	t.Helper()
	ip := outputs["ip"]
	if net.ParseIP(ip) == nil {
		t.Fatalf("output ip = %q, want an IP address", ip)
	}
	if want := "https://" + ip + ":8443/api/v1"; outputs["api_url"] != want {
		t.Errorf("output api_url = %q, want %q", outputs["api_url"], want)
	}
	if want := "ssh " + sshUser + "@" + ip; outputs["ssh_command"] != want {
		t.Errorf("output ssh_command = %q, want %q", outputs["ssh_command"], want)
	}
	return ip
}

func checkCloudInit(t *testing.T, cloudInit string) {
	t.Helper()
	for _, want := range []string{"#cloud-config", "git clone " + repoURL + " /opt/ft_hackthon", "docker compose up -d"} {
		if !strings.Contains(cloudInit, want) {
			t.Errorf("cloud-init is missing %q:\n%s", want, cloudInit)
		}
	}
}

func checkPorts(t *testing.T, what string, got []string) {
	t.Helper()
	want := slices.Clone(wantPorts)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("%s opens ports %v, want %v", what, got, wantPorts)
	}
}

func getJSON(t *testing.T, client *http.Client, rawURL string, into any) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer fake")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", rawURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK && into != nil {
		if err := json.Unmarshal(body, into); err != nil {
			t.Fatalf("GET %s: decode: %v\n%s", rawURL, err, body)
		}
	}
	return resp.StatusCode
}

type ec2Client struct {
	t        *testing.T
	endpoint string
	region   string
}

func (c ec2Client) call(params url.Values, into any) {
	c.t.Helper()
	params.Set("Version", "2016-11-15")
	req, err := http.NewRequest(http.MethodPost, c.endpoint+"/", strings.NewReader(params.Encode()))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// floci routes on the SigV4 credential scope and does not verify signatures.
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=test/%s/%s/ec2/aws4_request, SignedHeaders=host, Signature=test",
		time.Now().UTC().Format("20060102"), c.region))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("EC2 %s: %v", params.Get("Action"), err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("EC2 %s: status %d\n%s", params.Get("Action"), resp.StatusCode, body)
	}
	if err := xml.Unmarshal(body, into); err != nil {
		c.t.Fatalf("EC2 %s: decode: %v\n%s", params.Get("Action"), err, body)
	}
}

type ec2Instance struct {
	ID    string `xml:"instanceId"`
	Type  string `xml:"instanceType"`
	Image string `xml:"imageId"`
	State string `xml:"instanceState>name"`
}

func (c ec2Client) instances(name string) []ec2Instance {
	var resp struct {
		Reservations []struct {
			Instances []ec2Instance `xml:"instancesSet>item"`
		} `xml:"reservationSet>item"`
	}
	c.call(url.Values{"Action": {"DescribeInstances"}, "Filter.1.Name": {"tag:Name"}, "Filter.1.Value.1": {name}}, &resp)
	var all []ec2Instance
	for _, r := range resp.Reservations {
		all = append(all, r.Instances...)
	}
	return all
}

func TestDeployAWS(t *testing.T) {
	ep := endpoint("FLOCI_AWS_ENDPOINT", "http://localhost:14566")
	const region = "eu-west-1"
	ec2 := ec2Client{t: t, endpoint: ep, region: region}
	s := newStack(t, "aws", map[string]string{
		"TF_VAR_floci_endpoint": ep,
		"TF_VAR_region":         region,
	})

	outputs := s.deploy()
	checkOutputs(t, outputs, "ubuntu")

	running := slices.DeleteFunc(ec2.instances("ft-hackthon"), func(i ec2Instance) bool { return i.State != "running" })
	if len(running) != 1 {
		t.Fatalf("want 1 running ft-hackthon instance, got %+v", running)
	}
	inst := running[0]
	if inst.Type != "t3.small" {
		t.Errorf("instance type = %q, want t3.small", inst.Type)
	}

	var userData struct {
		Value string `xml:"userData>value"`
	}
	ec2.call(url.Values{"Action": {"DescribeInstanceAttribute"}, "InstanceId": {inst.ID}, "Attribute": {"userData"}}, &userData)
	decoded, err := base64.StdEncoding.DecodeString(userData.Value)
	if err != nil {
		t.Fatalf("decode user data: %v", err)
	}
	checkCloudInit(t, string(decoded))

	var groups struct {
		Groups []struct {
			Permissions []struct {
				Protocol string   `xml:"ipProtocol"`
				FromPort string   `xml:"fromPort"`
				ToPort   string   `xml:"toPort"`
				CIDRs    []string `xml:"ipRanges>item>cidrIp"`
			} `xml:"ipPermissions>item"`
		} `xml:"securityGroupInfo>item"`
	}
	ec2.call(url.Values{"Action": {"DescribeSecurityGroups"}, "GroupName.1": {"ft-hackthon"}}, &groups)
	if len(groups.Groups) != 1 {
		t.Fatalf("want 1 ft-hackthon security group, got %d", len(groups.Groups))
	}
	var ports []string
	for _, p := range groups.Groups[0].Permissions {
		if p.Protocol == "tcp" && p.FromPort == p.ToPort && slices.Contains(p.CIDRs, "0.0.0.0/0") {
			ports = append(ports, p.FromPort)
		}
	}
	checkPorts(t, "security group", ports)

	s.destroy()
	for _, i := range ec2.instances("ft-hackthon") {
		if i.ID == inst.ID && i.State != "terminated" {
			t.Errorf("instance %s is %q after destroy, want terminated", i.ID, i.State)
		}
	}
}

func TestDeployGCP(t *testing.T) {
	ep := endpoint("FLOCI_GCP_ENDPOINT", "http://localhost:14588")
	const project, zone = "floci-test", "europe-west1-b"
	publicKey := testPublicKey(t)
	s := newStack(t, "gcp", map[string]string{
		"TF_VAR_floci_endpoint": ep,
		"TF_VAR_project":        project,
		"TF_VAR_region":         "europe-west1",
		// floci-gcp's machine catalog is a small fixture without e2-small.
		"TF_VAR_size":               "e2-standard-2",
		"TF_VAR_ssh_key_name":       "ftuser",
		"TF_VAR_ssh_public_key":     publicKey,
		"GOOGLE_OAUTH_ACCESS_TOKEN": "fake-token",
	})
	compute := ep + "/compute/v1/projects/" + project

	outputs := s.deploy()
	ip := checkOutputs(t, outputs, "ftuser")
	if _, docRange, _ := net.ParseCIDR("203.0.113.0/24"); !docRange.Contains(net.ParseIP(ip)) {
		t.Errorf("external IP %s is not from floci-gcp's 203.0.113.0/24 pool", ip)
	}

	var inst struct {
		Status      string `json:"status"`
		MachineType string `json:"machineType"`
		Metadata    struct {
			Items []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"items"`
		} `json:"metadata"`
		NICs []struct {
			AccessConfigs []struct {
				NatIP string `json:"natIP"`
			} `json:"accessConfigs"`
		} `json:"networkInterfaces"`
	}
	instanceURL := compute + "/zones/" + zone + "/instances/ft-hackthon"
	if code := getJSON(t, http.DefaultClient, instanceURL, &inst); code != http.StatusOK {
		t.Fatalf("GET instance: status %d", code)
	}
	if inst.Status != "RUNNING" {
		t.Errorf("instance status = %q, want RUNNING", inst.Status)
	}
	if !strings.HasSuffix(inst.MachineType, "/machineTypes/e2-standard-2") {
		t.Errorf("machine type = %q, want e2-standard-2", inst.MachineType)
	}
	if len(inst.NICs) != 1 || len(inst.NICs[0].AccessConfigs) != 1 || inst.NICs[0].AccessConfigs[0].NatIP != ip {
		t.Errorf("instance NAT IP does not match output ip %s: %+v", ip, inst.NICs)
	}
	meta := map[string]string{}
	for _, item := range inst.Metadata.Items {
		meta[item.Key] = item.Value
	}
	checkCloudInit(t, meta["user-data"])
	if meta["ssh-keys"] != "ftuser:"+publicKey {
		t.Errorf("ssh-keys metadata = %q", meta["ssh-keys"])
	}

	var fw struct {
		SourceRanges []string `json:"sourceRanges"`
		Allowed      []struct {
			Protocol string   `json:"IPProtocol"`
			Ports    []string `json:"ports"`
		} `json:"allowed"`
	}
	if code := getJSON(t, http.DefaultClient, compute+"/global/firewalls/ft-hackthon", &fw); code != http.StatusOK {
		t.Fatalf("GET firewall: status %d", code)
	}
	if !slices.Equal(fw.SourceRanges, []string{"0.0.0.0/0"}) {
		t.Errorf("firewall source ranges = %v", fw.SourceRanges)
	}
	var ports []string
	for _, a := range fw.Allowed {
		if a.Protocol == "tcp" {
			ports = append(ports, a.Ports...)
		}
	}
	checkPorts(t, "firewall", ports)

	s.destroy()
	if code := getJSON(t, http.DefaultClient, instanceURL, nil); code != http.StatusNotFound {
		t.Errorf("GET instance after destroy: status %d, want 404", code)
	}
}

const azureSubscription = "00000000-0000-0000-0000-000000000001"

// trustFlociAz downloads floci-az's self-signed certificate and returns a CA
// bundle (system roots + that cert) for SSL_CERT_FILE. azurerm only talks to
// the emulator over HTTPS, and this avoids touching the system trust store.
func trustFlociAz(t *testing.T, ep string) string {
	t.Helper()
	resp, err := http.Get(ep + "/_floci/tls-cert")
	if err != nil {
		t.Fatalf("fetch floci-az TLS cert: %v", err)
	}
	defer resp.Body.Close()
	cert, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(cert), "BEGIN CERTIFICATE") {
		t.Fatalf("floci-az TLS cert unavailable (is FLOCI_AZ_TLS_ENABLED=true?): status %d\n%s", resp.StatusCode, cert)
	}
	var roots []byte
	for _, p := range []string{"/etc/ssl/certs/ca-certificates.crt", "/etc/pki/tls/certs/ca-bundle.crt", "/etc/ssl/cert.pem"} {
		if roots, err = os.ReadFile(p); err == nil {
			break
		}
	}
	bundle := filepath.Join(t.TempDir(), "ca-bundle.pem")
	if err := os.WriteFile(bundle, append(append(roots, '\n'), cert...), 0o644); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestDeployAzure(t *testing.T) {
	ep := endpoint("FLOCI_AZ_ENDPOINT", "http://localhost:14577")
	host := strings.TrimPrefix(strings.TrimPrefix(ep, "http://"), "https://")
	s := newStack(t, "azure", map[string]string{
		"TF_VAR_floci_metadata_host": host,
		"TF_VAR_ssh_public_key":      testPublicKey(t),
		"SSL_CERT_FILE":              trustFlociAz(t, ep),
	})
	rg := ep + "/subscriptions/" + azureSubscription + "/resourceGroups/ft-hackthon"

	outputs := s.deploy()
	ip := checkOutputs(t, outputs, "ftuser")

	var vm struct {
		Properties struct {
			Hardware struct {
				Size string `json:"vmSize"`
			} `json:"hardwareProfile"`
			OS struct {
				AdminUser  string `json:"adminUsername"`
				CustomData string `json:"customData"`
			} `json:"osProfile"`
			Storage struct {
				Image struct {
					Publisher string `json:"publisher"`
					Offer     string `json:"offer"`
				} `json:"imageReference"`
			} `json:"storageProfile"`
		} `json:"properties"`
	}
	vmURL := rg + "/providers/Microsoft.Compute/virtualMachines/ft-hackthon?api-version=2024-03-01"
	if code := getJSON(t, http.DefaultClient, vmURL, &vm); code != http.StatusOK {
		t.Fatalf("GET VM: status %d", code)
	}
	p := vm.Properties
	if p.Hardware.Size != "Standard_B2als_v2" {
		t.Errorf("VM size = %q, want Standard_B2als_v2", p.Hardware.Size)
	}
	if p.OS.AdminUser != "ftuser" {
		t.Errorf("admin user = %q, want ftuser", p.OS.AdminUser)
	}
	if p.Storage.Image.Publisher != "Canonical" || p.Storage.Image.Offer != "ubuntu-24_04-lts" {
		t.Errorf("image = %+v, want Canonical ubuntu-24_04-lts", p.Storage.Image)
	}
	if p.OS.CustomData != "" {
		decoded, err := base64.StdEncoding.DecodeString(p.OS.CustomData)
		if err != nil {
			t.Fatalf("decode custom data: %v", err)
		}
		checkCloudInit(t, string(decoded))
	}

	var pip struct {
		Properties struct {
			IP string `json:"ipAddress"`
		} `json:"properties"`
	}
	if code := getJSON(t, http.DefaultClient, rg+"/providers/Microsoft.Network/publicIPAddresses/ft-hackthon-ip?api-version=2024-01-01", &pip); code != http.StatusOK {
		t.Fatalf("GET public IP: status %d", code)
	}
	if pip.Properties.IP != ip {
		t.Errorf("public IP = %q, want output ip %q", pip.Properties.IP, ip)
	}

	var nsg struct {
		Properties struct {
			Rules []struct {
				Properties struct {
					Direction string `json:"direction"`
					Access    string `json:"access"`
					Protocol  string `json:"protocol"`
					Port      string `json:"destinationPortRange"`
				} `json:"properties"`
			} `json:"securityRules"`
		} `json:"properties"`
	}
	if code := getJSON(t, http.DefaultClient, rg+"/providers/Microsoft.Network/networkSecurityGroups/ft-hackthon?api-version=2024-01-01", &nsg); code != http.StatusOK {
		t.Fatalf("GET NSG: status %d", code)
	}
	var ports []string
	for _, r := range nsg.Properties.Rules {
		if r.Properties.Direction == "Inbound" && r.Properties.Access == "Allow" && strings.EqualFold(r.Properties.Protocol, "Tcp") {
			ports = append(ports, r.Properties.Port)
		}
	}
	checkPorts(t, "network security group", ports)

	s.destroy()
	if code := getJSON(t, http.DefaultClient, rg+"?api-version=2021-04-01", nil); code != http.StatusNotFound {
		t.Errorf("GET resource group after destroy: status %d, want 404", code)
	}
}
