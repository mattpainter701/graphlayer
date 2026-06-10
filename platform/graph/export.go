package graph

import (
	"fmt"
)

// Export serializes a TopologyGraph to the requested format.
func Export(g TopologyGraph) ([]byte, error) {
	switch g.Options.Format {
	case FormatGraphML:
		return ExportGraphML(g)
	case FormatGEXF:
		return ExportGEXF(g)
	case FormatCytoscape:
		return ExportCytoscape(g, true) // vendor-colored by default
	case FormatD3:
		return ExportD3(g)
	case FormatDOT:
		return ExportDOT(g)
	default:
		return nil, fmt.Errorf("graph: unsupported export format %q", g.Options.Format)
	}
}

// ExportToHTML exports the graph as a self-contained HTML visualization.
// Currently supports D3 format.
func ExportToHTML(g TopologyGraph) ([]byte, error) {
	switch g.Options.Format {
	case FormatD3:
		jsonData, err := ExportD3(g)
		if err != nil {
			return nil, err
		}
		return []byte(D3HTMLTemplate(jsonData)), nil
	default:
		return nil, fmt.Errorf("graph: HTML export only supports d3 format, got %q", g.Options.Format)
	}
}

// QuickExport is a convenience method that builds and exports a graph in one call.
func QuickExport(builder *GraphBuilder, format ExportFormat, title string) ([]byte, error) {
	builder.options.Format = format
	builder.options.Title = title
	graph := builder.Build()
	return Export(graph)
}

// SupportedFormats returns the list of supported export formats.
func SupportedFormats() []ExportFormat {
	return []ExportFormat{FormatGraphML, FormatGEXF, FormatCytoscape, FormatD3, FormatDOT}
}

// SupportedViewFilters returns the list of supported view filters.
func SupportedViewFilters() []ViewFilter {
	return []ViewFilter{ViewAll, ViewRoutingOnly, ViewAccessOnly, ViewSecurityOnly, ViewCoreOnly, ViewFirewallsOnly, ViewSwitchesOnly}
}

// VendorColorMap returns a copy of the vendor color mapping.
func VendorColorMap() map[string]string {
	out := make(map[string]string, len(VendorColors))
	for k, v := range VendorColors {
		out[k] = v
	}
	return out
}
