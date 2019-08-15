package public

import (
	"context"
	"testing"

	"github.com/golang/protobuf/proto"
	pb "github.com/linkerd/linkerd2/controller/gen/public"
	pkgK8s "github.com/linkerd/linkerd2/pkg/k8s"
	"github.com/prometheus/common/model"
)

const (
	clientIDLabel = model.LabelName("client_id")
	serverIDLabel = model.LabelName("server_id")
)

// response content for SRC, DST, SRC_NS, DST_NS, CLIENT_ID, SERVER_ID and MSG
var (
	resSrc = []string{
		"web",
		"web",
	}
	resDst = []string{
		"emoji",
		"voting",
	}
	resSrcNamespace = []string{
		"emojivoto",
		"emojivoto",
	}
	resDstNamespace = []string{
		"emojivoto",
		"emojivoto",
	}
	resClient = []string{
		"web.emojivoto.serviceaccount.identity.linkerd.cluster.local",
		"web.emojivoto.serviceaccount.identity.linkerd.cluster.local",
	}
	resServer = []string{
		"emoji.emojivoto.serviceaccount.identity.linkerd.cluster.local",
		"voting.emojivoto.serviceaccount.identity.linkerd.cluster.local",
	}
	resMsg = []string{"", ""}
)

type edgesExpected struct {
	expectedStatRPC
	req              pb.EdgesRequest  // the request we would like to test
	expectedResponse pb.EdgesResponse // the edges response we expect
}

func genInboundPromSample(resourceType, resourceName, clientID string) *model.Sample {
	resourceLabel := model.LabelName(resourceType)

	return &model.Sample{
		Metric: model.Metric{
			resourceLabel:  model.LabelValue(resourceName),
			namespaceLabel: model.LabelValue("emojivoto"),
			clientIDLabel:  model.LabelValue(clientID),
		},
		Value:     123,
		Timestamp: 456,
	}
}

func genOutboundPromSample(resourceType, resourceName, resourceNameDst, serverID string) *model.Sample {
	resourceLabel := model.LabelName(resourceType)
	dstResourceLabel := "dst_" + resourceLabel

	return &model.Sample{
		Metric: model.Metric{
			resourceLabel:     model.LabelValue(resourceName),
			namespaceLabel:    model.LabelValue("emojivoto"),
			dstNamespaceLabel: model.LabelValue("emojivoto"),
			dstResourceLabel:  model.LabelValue(resourceNameDst),
			serverIDLabel:     model.LabelValue(serverID),
		},
		Value:     123,
		Timestamp: 456,
	}
}

func testEdges(t *testing.T, expectations []edgesExpected) {
	for _, exp := range expectations {
		mockProm, fakeGrpcServer, err := newMockGrpcServer(exp.expectedStatRPC)
		if err != nil {
			t.Fatalf("Error creating mock grpc server: %s", err)
		}

		rsp, err := fakeGrpcServer.Edges(context.TODO(), &exp.req)
		if err != exp.err {
			t.Fatalf("Expected error: %s, Got: %s", exp.err, err)
		}

		err = exp.verifyPromQueries(mockProm)
		if err != nil {
			t.Fatal(err)
		}

		rspEdgeRows := rsp.GetOk().Edges

		if len(rspEdgeRows) != len(exp.expectedResponse.GetOk().Edges) {
			t.Fatalf(
				"Expected [%d] edge rows, got [%d].\nExpected:\n%s\nGot:\n%s",
				len(exp.expectedResponse.GetOk().Edges),
				len(rspEdgeRows),
				exp.expectedResponse.GetOk().Edges,
				rspEdgeRows,
			)
		}

		for i, st := range rspEdgeRows {
			expected := exp.expectedResponse.GetOk().Edges[i]
			if !proto.Equal(st, expected) {
				t.Fatalf("Expected: %+v\n Got: %+v\n", expected, st)
			}
		}

		if !proto.Equal(exp.expectedResponse.GetOk(), rsp.GetOk()) {
			t.Fatalf("Expected edgesOkResp: %+v\n Got: %+v", &exp.expectedResponse, rsp)
		}
	}
}

func TestEdges(t *testing.T) {
	t.Run("Successfully performs an edges query based on resource type Deployment", func(t *testing.T) {
		expectations := []edgesExpected{
			{
				expectedStatRPC: expectedStatRPC{
					err: nil,
					mockPromResponse: model.Vector{
						genInboundPromSample("deployment", "emoji", "web.emojivoto.serviceaccount.identity.linkerd.cluster.local"),
						genInboundPromSample("deployment", "voting", "web.emojivoto.serviceaccount.identity.linkerd.cluster.local"),
						genOutboundPromSample("deployment", "web", "emoji", "emoji.emojivoto.serviceaccount.identity.linkerd.cluster.local"),
						genOutboundPromSample("deployment", "web", "voting", "voting.emojivoto.serviceaccount.identity.linkerd.cluster.local"),
					},
				},
				req: pb.EdgesRequest{
					Selector: &pb.ResourceSelection{
						Resource: &pb.Resource{
							Namespace: "emojivoto",
							Type:      pkgK8s.Deployment,
						},
					},
				},
				expectedResponse: GenEdgesResponse("deployment", resSrc, resDst, resSrcNamespace, resDstNamespace, resClient, resServer, resMsg),
			}}

		testEdges(t, expectations)
	})
}
