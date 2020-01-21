package util

import (
	apipb "github.com/textileio/go-threads/service/api/pb"
	servicepb "github.com/textileio/go-threads/service/pb"
)

func RecFromServiceRec(r *servicepb.Log_Record) *apipb.Record {
	return &apipb.Record{
		RecordNode: r.RecordNode,
		EventNode:  r.EventNode,
		HeaderNode: r.HeaderNode,
		BodyNode:   r.BodyNode,
	}
}

func RecToServiceRec(r *apipb.Record) *servicepb.Log_Record {
	return &servicepb.Log_Record{
		RecordNode: r.RecordNode,
		EventNode:  r.EventNode,
		HeaderNode: r.HeaderNode,
		BodyNode:   r.BodyNode,
	}
}
