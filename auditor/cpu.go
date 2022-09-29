package auditor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/dustin/go-humanize/english"
)

func (a *appAuditor) cpu() {
	report := model.NewAuditReport(a.w.Ctx, "CPU")
	relevantNodes := map[string]*model.Node{}

	overloadedNodes := utils.NewStringSet()
	overloadedContainers := utils.NewStringSet()

	for _, i := range a.app.Instances {
		for _, c := range i.Containers {
			report.GetOrCreateChartInGroup("CPU usage of container <selector>, cores", c.Name).
				AddSeries(i.Name, c.CpuUsage).
				SetThreshold("limit", c.CpuLimit, timeseries.Max)
			report.GetOrCreateChartInGroup("CPU delay of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.CpuDelay)
			report.GetOrCreateChartInGroup("Throttled time of container <selector>, seconds/second", c.Name).AddSeries(i.Name, c.ThrottledTime)

			if c.CpuLimit != nil && c.CpuUsage != nil {
				usage := c.CpuUsage.Last() / c.CpuLimit.Last()
				if usage > a.getSimpleConfig(model.CheckIdContainerCPU, 80).Threshold {
					overloadedContainers.Add("%s@%s", c.Name, i.Name)
				}
			}
		}
		if node := i.Node; i.Node != nil {
			nodeName := node.Name.Value()
			if relevantNodes[nodeName] == nil {
				relevantNodes[nodeName] = i.Node
				report.GetOrCreateChartInGroup("Node CPU usage <selector>, %", "overview").
					AddSeries(nodeName, i.Node.CpuUsagePercent).
					Feature()

				if last := i.Node.CpuUsagePercent.Last(); last > a.getSimpleConfig(model.CheckIdNodeCPU, 80).Threshold {
					overloadedNodes.Add(i.Node.Name.Value())
				}

				byMode := report.GetOrCreateChartInGroup("Node CPU usage <selector>, %", nodeName).Sorted().Stacked()
				for _, s := range cpuByModeSeries(node.CpuUsageByMode) {
					byMode.Series = append(byMode.Series, s)
				}

				report.GetOrCreateChartInGroup("CPU consumers on <selector>, cores", nodeName).
					Stacked().
					Sorted().
					SetThreshold("total", node.CpuCapacity, timeseries.Any).
					AddMany(timeseries.Top(cpuConsumers(node), timeseries.NanSum, 5))
			}
		}
	}

	containerCPU := report.AddCheck(model.CheckIdContainerCPU)
	if overloadedContainers.Len() > 0 {
		containerCPU.SetStatus(
			model.WARNING,
			"high CPU usage of %s",
			english.Plural(overloadedContainers.Len(), "container", "containers"),
		)
	}

	nodeCPU := report.AddCheck(model.CheckIdNodeCPU)
	if overloadedNodes.Len() > 0 {
		nodeCPU.SetStatus(
			model.WARNING,
			"high CPU utilization of %s",
			english.Plural(overloadedNodes.Len(), "node", "nodes"),
		)
	}
	a.addReport(report)
}