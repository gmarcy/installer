package validation

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/utils/openstack/clientconfig"
	flavorutils "github.com/gophercloud/utils/openstack/compute/v2/flavors"
	networkutils "github.com/gophercloud/utils/openstack/networking/v2/networks"
	"github.com/pkg/errors"

	"github.com/openshift/installer/pkg/types"
)

// CloudInfo caches data fetched from the user's openstack cloud
type CloudInfo struct {
	ExternalNetwork *networks.Network
	Flavors         map[string]*flavors.Flavor
	MachinesSubnet  *subnets.Subnet
	APIFIP          *floatingips.FloatingIP
	IngressFIP      *floatingips.FloatingIP
	Zones           []string

	clients *clients
}

type clients struct {
	networkClient *gophercloud.ServiceClient
	computeClient *gophercloud.ServiceClient
}

// GetCloudInfo fetches and caches metadata from openstack
func GetCloudInfo(ic *types.InstallConfig) (*CloudInfo, error) {
	var err error
	ci := CloudInfo{
		clients: &clients{},
		Flavors: map[string]*flavors.Flavor{},
	}

	opts := &clientconfig.ClientOpts{Cloud: ic.OpenStack.Cloud}

	ci.clients.networkClient, err = clientconfig.NewServiceClient("network", opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a network client")
	}

	ci.clients.computeClient, err = clientconfig.NewServiceClient("compute", opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a compute client")
	}

	err = ci.collectInfo(ic)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate OpenStack cloud info")
	}

	return &ci, nil
}

func (ci *CloudInfo) collectInfo(ic *types.InstallConfig) error {
	var err error

	ci.ExternalNetwork, err = ci.getNetwork(ic.OpenStack.ExternalNetwork)
	if err != nil {
		return errors.Wrap(err, "failed to fetch external network info")
	}

	ci.Flavors[ic.OpenStack.FlavorName], err = ci.getFlavor(ic.OpenStack.FlavorName)
	if err != nil {
		return errors.Wrap(err, "failed to fetch platform flavor info")
	}

	if ic.ControlPlane != nil && ic.ControlPlane.Platform.OpenStack != nil {
		crtlPlaneFlavor := ic.ControlPlane.Platform.OpenStack.FlavorName
		if crtlPlaneFlavor != "" {
			ci.Flavors[crtlPlaneFlavor], err = ci.getFlavor(crtlPlaneFlavor)
			if err != nil {
				return err
			}
		}
	}

	for _, machine := range ic.Compute {
		if machine.Platform.OpenStack != nil {
			flavorName := machine.Platform.OpenStack.FlavorName
			if flavorName != "" {
				if _, seen := ci.Flavors[flavorName]; !seen {
					ci.Flavors[flavorName], err = ci.getFlavor(flavorName)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	ci.MachinesSubnet, err = ci.getSubnet(ic.OpenStack.MachinesSubnet)
	if err != nil {
		return errors.Wrap(err, "failed to fetch machine subnet info")
	}

	ci.APIFIP, err = ci.getFloatingIP(ic.OpenStack.LbFloatingIP)
	if err != nil {
		return err
	}

	ci.IngressFIP, err = ci.getFloatingIP(ic.OpenStack.IngressFloatingIP)
	if err != nil {
		return err
	}

	ci.Zones, err = ci.getZones()
	if err != nil {
		return err
	}

	return nil
}

func (ci *CloudInfo) getSubnet(subnetID string) (*subnets.Subnet, error) {
	subnet, err := subnets.Get(ci.clients.networkClient, subnetID).Extract()
	if err != nil {
		var gerr *gophercloud.ErrResourceNotFound
		if errors.As(err, &gerr) {
			return nil, nil
		}
		return nil, err
	}

	return subnet, nil
}

func (ci *CloudInfo) getFlavor(flavorName string) (*flavors.Flavor, error) {
	flavorID, err := flavorutils.IDFromName(ci.clients.computeClient, flavorName)
	if err != nil {
		var gerr *gophercloud.ErrResourceNotFound
		if errors.As(err, &gerr) {
			return nil, nil
		}
		return nil, err
	}

	flavor, err := flavors.Get(ci.clients.computeClient, flavorID).Extract()
	if err != nil {
		return nil, err
	}

	return flavor, nil
}

func (ci *CloudInfo) getNetwork(networkName string) (*networks.Network, error) {
	if networkName == "" {
		return nil, nil
	}
	networkID, err := networkutils.IDFromName(ci.clients.networkClient, networkName)
	if err != nil {
		var gerr *gophercloud.ErrResourceNotFound
		if errors.As(err, &gerr) {
			return nil, nil
		}
		return nil, err
	}

	network, err := networks.Get(ci.clients.networkClient, networkID).Extract()
	if err != nil {
		return nil, err
	}

	return network, nil
}

func (ci *CloudInfo) getFloatingIP(fip string) (*floatingips.FloatingIP, error) {
	if fip != "" {
		opts := floatingips.ListOpts{
			FloatingIP: fip,
		}
		allPages, err := floatingips.List(ci.clients.networkClient, opts).AllPages()
		if err != nil {
			return nil, err
		}

		allFIPs, err := floatingips.ExtractFloatingIPs(allPages)
		if err != nil {
			return nil, err
		}

		if len(allFIPs) == 0 {
			return nil, nil
		}
		return &allFIPs[0], nil
	}
	return nil, nil
}

func (ci *CloudInfo) getZones() ([]string, error) {
	zones := []string{}
	allPages, err := availabilityzones.List(ci.clients.computeClient).AllPages()
	if err != nil {
		return nil, err
	}

	availabilityZoneInfo, err := availabilityzones.ExtractAvailabilityZones(allPages)
	if err != nil {
		return nil, err
	}

	for _, zoneInfo := range availabilityZoneInfo {
		if zoneInfo.ZoneState.Available {
			zones = append(zones, zoneInfo.ZoneName)
		}
	}

	if len(zones) == 0 {
		return nil, errors.New("could not find an available compute availability zone")
	}

	return zones, nil
}
