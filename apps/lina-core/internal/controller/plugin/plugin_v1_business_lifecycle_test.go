package plugin

import (
	"context"
	"testing"

	v1 "lina-core/api/plugin/v1"
	pluginsvc "lina-core/internal/service/plugin"
	"lina-core/pkg/plugin/pluginhost"
)

type pluginBusinessLifecycleFakeService struct {
	pluginsvc.Service
	stoppedID string
	resumedID string
	report    *pluginsvc.BusinessImpactReport
}

func (f *pluginBusinessLifecycleFakeService) StopNewBusiness(_ context.Context, pluginID string) error {
	f.stoppedID = pluginID
	return nil
}

func (f *pluginBusinessLifecycleFakeService) ResumeNewBusiness(_ context.Context, pluginID string) error {
	f.resumedID = pluginID
	return nil
}

func (f *pluginBusinessLifecycleFakeService) CheckBusinessImpact(
	_ context.Context,
	pluginID string,
) (*pluginsvc.BusinessImpactReport, error) {
	if f.report == nil {
		return &pluginsvc.BusinessImpactReport{PluginID: pluginID}, nil
	}
	return f.report, nil
}

func TestBusinessLifecycleControllerMappings(t *testing.T) {
	fake := &pluginBusinessLifecycleFakeService{report: &pluginsvc.BusinessImpactReport{
		PluginID: "ssl-provider", BlockingCount: 2,
		Items: []pluginsvc.BusinessImpactItem{{
			OwnerPluginID: "commerce-core",
			Category:      pluginhost.BusinessImpactActiveServices,
			TotalCount:    4,
			BlockingCount: 2,
		}},
	}}
	controller := &ControllerV1{pluginSvc: fake}
	ctx := context.Background()

	stop, err := controller.StopNewBusiness(ctx, &v1.StopNewBusinessReq{Id: "ssl-provider"})
	if err != nil || fake.stoppedID != "ssl-provider" || stop.NewBusinessState != v1.NewBusinessStateStopped {
		t.Fatalf("unexpected stop mapping: response=%#v service=%#v err=%v", stop, fake, err)
	}
	resume, err := controller.ResumeNewBusiness(ctx, &v1.ResumeNewBusinessReq{Id: "ssl-provider"})
	if err != nil || fake.resumedID != "ssl-provider" || resume.NewBusinessState != v1.NewBusinessStateAccepting {
		t.Fatalf("unexpected resume mapping: response=%#v service=%#v err=%v", resume, fake, err)
	}
	impact, err := controller.BusinessImpact(ctx, &v1.BusinessImpactReq{Id: "ssl-provider"})
	if err != nil || impact.CanDisable || impact.CanUninstall || impact.BlockingCount != 2 || len(impact.Items) != 1 {
		t.Fatalf("unexpected impact mapping: response=%#v err=%v", impact, err)
	}
}
