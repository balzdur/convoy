package services

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideEventService(ctrl *gomock.Controller) (*EventService, error) {
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	cache := mocks.NewMockCache(ctrl)
	searcher := mocks.NewMockSearcher(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	deviceRepo := mocks.NewMockDeviceRepository(ctrl)

	return &EventService{
		endpointRepo:      endpointRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		subRepo:           subRepo,
		sourceRepo:        sourceRepo,
		deviceRepo:        deviceRepo,
		queue:             queue,
		cache:             cache,
		searcher:          searcher,
	}, nil
}

func TestEventService_CreateEvent(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx        context.Context
		newMessage *models.Event
		g          *datastore.Project
	}
	tests := []struct {
		name        string
		dbFn        func(es *EventService)
		args        args
		wantEvent   *datastore.Event
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_event",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:     &datastore.SignatureConfiguration{},
						ReplayAttacks: false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Raw:              `{"name":"convoy"}`,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				ProjectID:        "abc",
			},
		},

		{
			name: "should_create_event_with_exponential_backoff_strategy",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "exponential",
							Duration:   1000,
							RetryCount: 10,
						},
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Raw:              `{"name":"convoy"}`,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				ProjectID:        "abc",
			},
		},
		{
			name: "should_create_event_for_disabled_endpoint",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:     &datastore.SignatureConfiguration{},
						ReplayAttacks: false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Raw:              `{"name":"convoy"}`,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				ProjectID:        "abc",
			},
		},
		{
			name: "should_create_event_with_custom_headers",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID:    "123",
					EventType:     "payment.created",
					Data:          bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
					CustomHeaders: map[string]string{"X-Test-Signature": "Test"},
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:     &datastore.SignatureConfiguration{},
						ReplayAttacks: false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Raw:              `{"name":"convoy"}`,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				ProjectID:        "abc",
				Headers:          httpheader.HTTPHeader{"X-Test-Signature": []string{"Test"}},
			},
		},
		{
			name: "should_error_for_invalid_strategy_config",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:    "abc",
					Name:   "test_project",
					Config: &datastore.ProjectConfig{},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "retry strategy not defined in configuration",
		},
		{
			name: "should_error_for_empty_endpoints",
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID: "",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  ErrInvalidEndpointID.Error(),
		},
		{
			name: "should_error_for_endpoint_not_found",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(nil, datastore.ErrEndpointNotFound)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  datastore.ErrEndpointNotFound.Error(),
		},

		{
			name: "should_fail_to_create_event",
			dbFn: func(es *EventService) {},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while creating event - invalid project",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			event, err := es.CreateEvent(tc.args.ctx, tc.args.newMessage, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, event.UID)
			require.NotEmpty(t, event.CreatedAt)
			require.NotEmpty(t, event.UpdatedAt)
			require.Empty(t, event.DeletedAt)

			stripVariableFields(t, "event", event)

			m1 := tc.wantEvent.Endpoints[0]
			m2 := event.Endpoints[0]

			tc.wantEvent.Endpoints[0], event.Endpoints[0] = "", ""
			require.Equal(t, tc.wantEvent, event)
			require.Equal(t, m1, m2)
		})
	}
}

func TestEventService_CreateFanoutEvent(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx        context.Context
		newMessage *models.FanoutEvent
		g          *datastore.Project
	}

	tests := []struct {
		name        string
		dbFn        func(es *EventService)
		args        args
		wantEvent   *datastore.Event
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_fanout_event_for_multiple_endpoints",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByOwnerID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{
					{
						Title:        "test_app",
						UID:          "123",
						ProjectID:    "abc",
						SupportEmail: "test_app@gmail.com",
					},

					{
						Title:        "test_app",
						UID:          "12345",
						ProjectID:    "abc",
						SupportEmail: "test_app@gmail.com",
					},
				}, nil)
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.FanoutEvent{
					OwnerID:   "12345",
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:     &datastore.SignatureConfiguration{},
						ReplayAttacks: false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Raw:              `{"name":"convoy"}`,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123", "12345"},
				ProjectID:        "abc",
			},
		},

		{
			name: "should_error_for_empty_endpoints",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByOwnerID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{}, nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.FanoutEvent{
					OwnerID:   "12345",
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  ErrNoValidOwnerIDEndpointFound.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			event, err := es.CreateFanoutEvent(tc.args.ctx, tc.args.newMessage, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, event.UID)
			require.NotEmpty(t, event.CreatedAt)
			require.NotEmpty(t, event.UpdatedAt)
			require.Empty(t, event.DeletedAt)

			stripVariableFields(t, "event", event)

			m1 := tc.wantEvent.Endpoints[0]
			m2 := event.Endpoints[0]

			tc.wantEvent.Endpoints[0], event.Endpoints[0] = "", ""
			require.Equal(t, tc.wantEvent, event)
			require.Equal(t, m1, m2)
		})
	}
}

func TestEventService_ReplayAppEvent(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx   context.Context
		event *datastore.Event
		g     *datastore.Project
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(es *EventService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_replay_app_event",
			args: args{
				ctx:   ctx,
				event: &datastore.Event{UID: "123"},
				g:     &datastore.Project{UID: "123", Name: "test_project"},
			},
			dbFn: func(es *EventService) {
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_replay_app_event",
			args: args{
				ctx:   ctx,
				event: &datastore.Event{UID: "123"},
				g:     &datastore.Project{UID: "123", Name: "test_project"},
			},
			dbFn: func(es *EventService) {
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to write event to queue",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.ReplayEvent(tc.args.ctx, tc.args.event, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_BatchRetryEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name          string
		args          args
		dbFn          func(es *EventService)
		wantSuccesses int
		wantFailures  int
		wantErr       bool
		wantErrCode   int
		wantErrMsg    string
	}{
		{
			name: "should_batch_retry_event_deliveries",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
					Pageable: datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					Status: []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
				},
			},
			wantSuccesses: 2,
			wantFailures:  0,
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ss, _ := es.endpointRepo.(*mocks.MockEndpointRepository)

				ss.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "123").
					Return(&datastore.Endpoint{
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(2)

				ed.EXPECT().LoadEventDeliveriesPaged(
					gomock.Any(),
					"123",
					[]string{"abc"},
					"13429",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
					datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					}).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID:            "ref",
								SubscriptionID: "sub-1",
							},
							{
								UID:            "oop",
								SubscriptionID: "sub-2",
								Status:         datastore.FailureEventStatus,
							},
						},
						datastore.PaginationData{},
						nil,
					)

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)
			},
		},
		{
			name: "should_batch_retry_event_deliveries_with_one_failure",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
					Pageable: datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					Status: []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ss, _ := es.endpointRepo.(*mocks.MockEndpointRepository)

				ss.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "123").
					Return(&datastore.Endpoint{
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

				ed.EXPECT().LoadEventDeliveriesPaged(
					gomock.Any(),
					"123",
					[]string{"abc"},
					"13429",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
					datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					}).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID:            "ref",
								SubscriptionID: "sub-1",
								Status:         datastore.SuccessEventStatus,
							},
							{
								UID:            "oop",
								SubscriptionID: "sub-2",
								Status:         datastore.FailureEventStatus,
							},
						},
						datastore.PaginationData{},
						nil,
					)

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantSuccesses: 1,
			wantFailures:  1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			successes, failures, err := es.BatchRetryEventDelivery(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantSuccesses, successes)
			require.Equal(t, tc.wantFailures, failures)
		})
	}
}

func TestEventService_ForceResendEventDeliveries(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		ids []string
		g   *datastore.Project
	}
	tests := []struct {
		name          string
		args          args
		dbFn          func(es *EventService)
		wantSuccesses int
		wantFailures  int
		wantErr       bool
		wantErrCode   int
		wantErrMsg    string
	}{
		{
			name: "should_force_resend_event_deliveries",
			args: args{
				ctx: ctx,
				ids: []string{"oop", "ref"},
				g:   &datastore.Project{UID: "123"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().FindEventDeliveriesByIDs(
					gomock.Any(), gomock.Any(), []string{"oop", "ref"}).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID: "ref",

								Status: datastore.SuccessEventStatus,
							},
							{
								UID:    "oop",
								Status: datastore.SuccessEventStatus,
							},
						},
						nil,
					)

				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "123").
					Times(2).Return(&datastore.Endpoint{
					Status: datastore.ActiveEndpointStatus,
				}, nil)

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)
			},
			wantSuccesses: 2,
			wantFailures:  0,
		},
		{
			name: "should_fail_validation_for_resend_event_deliveries_with_one_failure",
			args: args{
				ctx: ctx,
				ids: []string{"ref", "oop"},
				g:   &datastore.Project{UID: "123"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().FindEventDeliveriesByIDs(
					gomock.Any(), gomock.Any(), []string{"ref", "oop"}).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID:    "ref",
								Status: datastore.SuccessEventStatus,
							},
							{
								UID:    "oop",
								Status: datastore.FailureEventStatus,
							},
						},
						nil,
					)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  ErrInvalidEventDeliveryStatus.Error(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			successes, failures, err := es.ForceResendEventDeliveries(tc.args.ctx, tc.args.ids, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantSuccesses, successes)
			require.Equal(t, tc.wantFailures, failures)
		})
	}
}

func TestEventService_SearchEvents(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name               string
		args               args
		dbFn               func(es *EventService)
		wantEvents         []datastore.Event
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_get_event_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:    &datastore.Project{UID: "123"},
					EndpointID: "abc",
					Pageable: datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				se, _ := es.searcher.(*mocks.MockSearcher)
				se.EXPECT().Search(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]string{"1234"}, datastore.PaginationData{
						PerPage: 2,
					}, nil)

				ed, _ := es.eventRepo.(*mocks.MockEventRepository)
				ed.EXPECT().FindEventsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]datastore.Event{{UID: "1234"}}, nil)
			},
			wantEvents: []datastore.Event{
				{UID: "1234"},
			},
			wantPaginationData: datastore.PaginationData{
				PerPage: 2,
			},
		},
		{
			name: "should_fail_to_get_events_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:    &datastore.Project{UID: "123"},
					EndpointID: "abc",
					EventID:    "ref",
					Status:     []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.searcher.(*mocks.MockSearcher)
				ed.EXPECT().
					Search(gomock.Any(), gomock.Any()).
					Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			events, paginationData, err := es.Search(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEvents, events)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}

func TestEventService_ResendEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Project
	}
	tests := []struct {
		name       string
		dbFn       func(es *EventService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_retry_event_delivery",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{Status: datastore.ActiveEndpointStatus}, nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
		},
		{
			name: "should_error_for_success_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "event already sent",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.ResendEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_RetryEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Project
	}
	tests := []struct {
		name       string
		dbFn       func(es *EventService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_retry_event_delivery",
			dbFn: func(es *EventService) {
				er, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				er.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{Status: datastore.ActiveEndpointStatus}, nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
		},
		{
			name: "should_error_for_success_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "event already sent",
		},
		{
			name: "should_error_for_retry_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.RetryEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "cannot resend event that did not fail previously",
		},
		{
			name: "should_error_for_processing_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.ProcessingEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "cannot resend event that did not fail previously",
		},
		{
			name: "should_error_for_scheduled_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.ScheduledEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "cannot resend event that did not fail previously",
		},
		{
			name: "should_fail_to_find_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(nil, datastore.ErrEndpointNotFound)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "endpoint not found",
		},
		{
			name: "should_error_for_pending_subscription_status",
			dbFn: func(es *EventService) {
				s, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.PendingEndpointStatus,
				}, nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "endpoint is being re-activated",
		},
		{
			name: "should_retry_event_delivery_with_inactive_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.InactiveEndpointStatus,
				}, nil)

				s.EXPECT().UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.PendingEndpointStatus).
					Times(1).Return(nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
		},
		{
			name: "should_fail_to_retry_event_delivery_with_inactive_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.InactiveEndpointStatus,
				}, nil)

				s.EXPECT().UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.PendingEndpointStatus).
					Times(1).Return(errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "failed to update endpoint status",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.RetryEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_forceResendEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Project
	}
	tests := []struct {
		name       string
		dbFn       func(es *EventService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_force_resend_event_delivery",
			dbFn: func(es *EventService) {
				s, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "test_project").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.ActiveEndpointStatus,
				}, nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "test_project"},
			},
		},
		{
			name: "should_fail_to_find_endpoint",
			dbFn: func(es *EventService) {
				s, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "test_project").
					Times(1).Return(nil, errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "test_project"},
			},
			wantErr:    true,
			wantErrMsg: "endpoint not found",
		},
		{
			name: "should_error_not_active_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "test_project").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.InactiveEndpointStatus,
				}, nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "test_project"},
			},
			wantErr:    true,
			wantErrMsg: "force resend to an inactive or pending endpoint is not allowed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.forceResendEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_requeueEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Project
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(es *EventService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_requeue_event_delivery",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Project{Name: "test_project"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_fail_update_event_delivery_status",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Project{Name: "test_project"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while trying to resend event",
		},
		{
			name: "should_fail_to_write_event_delivery_to_queue",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Project{Name: "test_project"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "error occurred re-enqueing old event - 123: failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.requeueEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_CreateDynamicEvents(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx          context.Context
		dynamicEvent *models.DynamicEvent
		g            *datastore.Project
	}
	tests := []struct {
		name        string
		dbFn        func(es *EventService)
		args        args
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_dynamic_event",
			dbFn: func(es *EventService) {
				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.CreateDynamicEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				dynamicEvent: &models.DynamicEvent{
					Endpoint: models.DynamicEndpoint{
						URL:    "https://google.com",
						Secret: "abc",
						Name:   "test_endpoint",
					},
					Subscription: models.DynamicSubscription{
						Name:            "test-sub",
						AlertConfig:     nil,
						RetryConfig:     nil,
						FilterConfig:    nil,
						RateLimitConfig: nil,
					},
					Event: models.DynamicEventStub{
						EventType: "*",
						Data:      []byte(`{"name":"daniel"}`),
						CustomHeaders: map[string]string{
							"X-signature": "HEX",
						},
					},
				},
				g: &datastore.Project{UID: "12345"},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_nil_project",
			dbFn: func(es *EventService) {},
			args: args{
				ctx:          ctx,
				dynamicEvent: &models.DynamicEvent{},
				g:            nil,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while creating event - invalid project",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es, err := provideEventService(ctrl)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.CreateDynamicEvent(tc.args.ctx, tc.args.dynamicEvent, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
