package cronet

import (
	"context"

	mDNS "github.com/miekg/dns"
)

// DNSResolverFunc resolves a DNS request into a DNS response.
//
// The resolver is used by NaiveClient's optional in-process DNS server. The
// returned message should be a response to the request; the implementation
// will normalize the ID and question section as needed.
type DNSResolverFunc func(ctx context.Context, request *mDNS.Msg) (response *mDNS.Msg)

