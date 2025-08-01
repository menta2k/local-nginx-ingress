package docker

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// ContainerData represents a Docker container with nginx ingress configuration
type ContainerData struct {
	Config      *ContainerConfig
	IPAddress   string
	NetworkName string
	Status      string
}

// ListContainers retrieves all containers and extracts nginx ingress configurations
func ListContainers(ctx context.Context, cli *client.Client) ([]*ContainerData, error) {
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All: false, // Only running containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var containerData []*ContainerData

	for _, container := range containers {
		// Skip containers without nginx ingress labels
		if !hasNginxLabels(container.Labels) {
			continue
		}

		// Get container details
		containerJSON, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			fmt.Printf("Warning: failed to inspect container %s: %v\n", container.ID, err)
			continue
		}

		// Extract network information
		networkIP, networkName := extractNetworkInfo(containerJSON)

		// Extract nginx configuration from labels
		config, err := ExtractConfig(container.ID, getContainerName(container.Names), networkIP, container.Labels)
		if err != nil {
			fmt.Printf("Warning: failed to extract config for container %s: %v\n", container.ID, err)
			continue
		}

		// Skip if nginx ingress is not enabled
		if !config.Enabled {
			continue
		}

		// Validate configuration
		if err := ValidateConfig(config); err != nil {
			fmt.Printf("Warning: invalid config for container %s: %v\n", container.ID, err)
			continue
		}

		data := &ContainerData{
			Config:      config,
			IPAddress:   networkIP,
			NetworkName: networkName,
			Status:      container.Status,
		}

		containerData = append(containerData, data)
	}

	return containerData, nil
}

// hasNginxLabels checks if container has any nginx ingress labels
func hasNginxLabels(labels map[string]string) bool {
	for key := range labels {
		if strings.HasPrefix(key, LabelPrefix) {
			return true
		}
	}
	return false
}

// extractNetworkInfo extracts network IP and network name from container
func extractNetworkInfo(containerJSON container.InspectResponse) (string, string) {
	networks := containerJSON.NetworkSettings.Networks
	
	// Priority order: custom networks first, then bridge
	var networkName, networkIP string
	
	// First try to find a custom network (not bridge)
	for name, network := range networks {
		if name != "bridge" && network.IPAddress != "" {
			return network.IPAddress, name
		}
	}
	
	// Fallback to bridge network
	if bridgeNetwork, exists := networks["bridge"]; exists && bridgeNetwork.IPAddress != "" {
		return bridgeNetwork.IPAddress, "bridge"
	}
	
	// If no IP found, try to get from NetworkSettings
	if containerJSON.NetworkSettings.IPAddress != "" {
		return containerJSON.NetworkSettings.IPAddress, "bridge"
	}
	
	return networkIP, networkName
}

// getContainerName extracts clean container name from names array
func getContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	
	// Container names start with '/', remove it
	name := names[0]
	if strings.HasPrefix(name, "/") {
		name = name[1:]
	}
	
	return name
}

// GetContainerIP gets the IP address of a specific container
func GetContainerIP(ctx context.Context, cli *client.Client, containerID string) (string, error) {
	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}
	
	ip, _ := extractNetworkInfo(containerJSON)
	if ip == "" {
		return "", fmt.Errorf("no IP address found for container %s", containerID)
	}
	
	return ip, nil
}

// IsContainerHealthy checks if container is healthy and reachable
func IsContainerHealthy(ctx context.Context, cli *client.Client, containerID string) bool {
	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false
	}
	
	// Check if container is running
	if containerJSON.State.Status != "running" {
		return false
	}
	
	// Check health status if available
	if containerJSON.State.Health != nil {
		return containerJSON.State.Health.Status == "healthy"
	}
	
	return true
}

// CheckContainerPort verifies if the specified port is accessible on the container
func CheckContainerPort(ctx context.Context, containerIP string, port int) bool {
	address := fmt.Sprintf("%s:%d", containerIP, port)
	
	conn, err := net.DialTimeout("tcp", address, 5) // 5 second timeout
	if err != nil {
		return false
	}
	defer conn.Close()
	
	return true
}

// FilterEnabledContainers filters containers that have nginx ingress enabled
func FilterEnabledContainers(containers []*ContainerData) []*ContainerData {
	var enabled []*ContainerData
	
	for _, container := range containers {
		if container.Config.Enabled {
			enabled = append(enabled, container)
		}
	}
	
	return enabled
}

// GroupContainersByHost groups containers by their host configuration
func GroupContainersByHost(containers []*ContainerData) map[string][]*ContainerData {
	hostGroups := make(map[string][]*ContainerData)
	
	for _, container := range containers {
		host := container.Config.Host
		hostGroups[host] = append(hostGroups[host], container)
	}
	
	return hostGroups
}