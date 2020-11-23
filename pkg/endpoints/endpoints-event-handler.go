package endpoints

import (
	"github.com/mcluseau/kube-proxy2/pkg/api/localnetv1"
	"github.com/mcluseau/kube-proxy2/pkg/proxystore"
	v1 "k8s.io/api/core/v1"
)

type endpointsEventHandler struct{ eventHandler }

func (h *endpointsEventHandler) sourceName(eps *v1.Endpoints) string {
	return "endpoints/" + eps.Name // ensure its different from any EndpointsSlice name
}

func (h *endpointsEventHandler) OnAdd(obj interface{}) {
	eps := obj.(*v1.Endpoints)

	sourceName := h.sourceName(eps)

	// update store
	h.s.Update(func(tx *proxystore.Tx) {
		// expensive update as we're computing endpoints here, but still the best we can do
		infos := make([]*localnetv1.EndpointInfo, 0)
		for _, subset := range eps.Subsets {
			// add endpoints
			for _, set := range []struct {
				ready     bool
				addresses []v1.EndpointAddress
			}{
				{true, subset.Addresses},
				{false, subset.NotReadyAddresses},
			} {
				for _, addr := range set.addresses {
					info := &localnetv1.EndpointInfo{
						Namespace:   eps.Namespace,
						ServiceName: eps.Name, // eps name is the service name
						SourceName:  sourceName,
						Endpoint: &localnetv1.Endpoint{
							Hostname: addr.Hostname,
						},
						Conditions: &localnetv1.EndpointConditions{
							Ready: set.ready,
						},
					}

					if addr.NodeName != nil && *addr.NodeName != "" {
						info.NodeName = *addr.NodeName

						node := tx.GetNode(info.NodeName)

						if node != nil {
							info.Topology = node.Labels
						}
					}

					if addr.IP != "" {
						info.Endpoint.AddAddress(addr.IP)
					}

					infos = append(infos, info)
				}
			}
		}

		tx.SetEndpointsOfSource(eps.Namespace, sourceName, infos)
		h.updateSync(proxystore.Endpoints, tx)
	})
}

func (h *endpointsEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h *endpointsEventHandler) OnDelete(oldObj interface{}) {
	eps := oldObj.(*v1.Endpoints)

	sourceName := h.sourceName(eps)

	// update store
	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelEndpointsOfSource(eps.Namespace, sourceName)
		h.updateSync(proxystore.Endpoints, tx)
	})
}