package nomad

import (
    "context"
    "encoding/json"
    "fmt"
    "net"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/coredns/coredns/plugin"
    "github.com/coredns/coredns/plugin/pkg/dnsutil"
    "github.com/miekg/dns"
)

// Plugin struct
type Nomad struct {
    Next       plugin.Handler
    Domain     string
    NomadAddr  string
    Cache      map[string][]net.IP
    CacheMutex sync.RWMutex
    CacheTTL   time.Duration
    CacheTime  map[string]time.Time
}

// Ensure Nomad implements plugin.Handler
var _ plugin.Handler = &Nomad{}

// ServeDNS handles DNS queries
func (n *Nomad) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
    state := plugin.State{W: w, Req: r}
    qname := state.Name()
    if !dnsutil.IsSubDomain(n.Domain, qname) {
        return plugin.NextOrFailure(n.Name(), n.Next, ctx, w, r)
    }

    service := strings.TrimSuffix(qname, "."+n.Domain)
    service = strings.TrimSuffix(service, ".") // remove trailing dot if any

    ips, err := n.lookupService(service)
    if err != nil {
        return dns.RcodeServerFailure, err
    }
    if len(ips) == 0 {
        return dns.RcodeNameError, nil // NXDOMAIN
    }

    m := new(dns.Msg)
    m.SetReply(r)
    m.Authoritative = true

    for _, ip := range ips {
        rr := &dns.A{
            Hdr: dns.RR_Header{
                Name:   qname,
                Rrtype: dns.TypeA,
                Class:  dns.ClassINET,
                Ttl:    30,
            },
            A: ip,
        }
        m.Answer = append(m.Answer, rr)
    }

    w.WriteMsg(m)
    return dns.RcodeSuccess, nil
}

// lookupService queries Nomad API for service IPs with caching
func (n *Nomad) lookupService(service string) ([]net.IP, error) {
    n.CacheMutex.RLock()
    ips, ok := n.Cache[service]
    t, timeOk := n.CacheTime[service]
    n.CacheMutex.RUnlock()

    if ok && timeOk && time.Since(t) < n.CacheTTL {
        return ips, nil
    }

    // Query Nomad API for service allocations
    url := fmt.Sprintf("%s/v1/health/service/%s", n.NomadAddr, service)
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result []struct {
        Service struct {
            Address string `json:"Address"`
        } `json:"Service"`
        Checks []struct {
            Status string `json:"Status"`
        } `json:"Checks"`
        Node struct {
            Address string `json:"Address"`
        } `json:"Node"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    var serviceIPs []net.IP
    for _, entry := range result {
        // Only include services with passing health
        healthy := true
        for _, check := range entry.Checks {
            if check.Status != "passing" {
                healthy = false
                break
            }
        }
        if !healthy {
            continue
        }
        ip := net.ParseIP(entry.Service.Address)
        if ip == nil {
            ip = net.ParseIP(entry.Node.Address)
        }
        if ip != nil {
            serviceIPs = append(serviceIPs, ip)
        }
    }

    // Update cache
    n.CacheMutex.Lock()
    n.Cache[service] = serviceIPs
    n.CacheTime[service] = time.Now()
    n.CacheMutex.Unlock()

    return serviceIPs, nil
}

func (n *Nomad) Name() string { return "nomad" }

