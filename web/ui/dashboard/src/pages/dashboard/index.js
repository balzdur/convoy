import React, { useEffect, useState, useCallback } from 'react';
import ArrowDownIcon from '../../assets/img/arrow-down-icon.svg';
import AppsIcon from '../../assets/img/apps-icon.svg';
import MessageIcon from '../../assets/img/message-icon.svg';
import RefreshIcon from '../../assets/img/refresh-icon.svg';
import CalendarIcon from '../../assets/img/calendar-icon.svg';
import LinkIcon from '../../assets/img/link-icon.svg';
import AngleArrowDownIcon from '../../assets/img/angle-arrow-down.svg';
import ConvoyLogo from '../../assets/img/logo.svg';
import CopyIcon from '../../assets/img/copy-icon.svg';
import RetryIcon from '../../assets/img/retry-icon.svg';
import EmptyStateImage from '../../assets/img/empty-state-img.svg';
import ViewEventsIcon from '../../assets/img/view-events-icon.svg';
import Chart from 'chart.js/auto';
import { DateRange } from 'react-date-range';
import { request } from '../../services/https.service';
import './style.scss';
import 'react-date-range/dist/styles.css';
import 'react-date-range/dist/theme/default.css';
import { showNotification } from '../../components/app-notification';
import { copyText, getDate, getTime, logout } from '../../helpers/common.helper';
import Prism from 'prismjs';
import '../../scss/prism.scss';
import '../../helpers/prism-line-plugin';

const moment = require('moment');
const authDetails = localStorage.getItem('CONVOY_AUTH');

function DashboardPage() {
	const [dashboardData, setDashboardData] = useState({ apps: 0, messages: 0, messageData: [] });
	const [showDropdown, toggleShowDropdown] = useState(false);
	const [apps, setAppsData] = useState({ content: [], pagination: { page: 1, totalPage: 0 } });
	const [events, setEventsData] = useState({ content: [], pagination: { page: 1, totalPage: 0 } });
	const [displayedEvents, setDisplayedEvents] = useState([]);
	const [tabs] = useState(['events', 'apps']);
	const [activeTab, setActiveTab] = useState('events');
	const [eventDetailsTabs] = useState([
		{ id: 'data', label: 'Event Data' },
		{ id: 'response', label: 'Response Header' },
		{ id: 'request', label: 'Request Header' }
	]);
	const [eventDetailsActiveTab, setEventDetailsActiveTab] = useState('data');
	const [showFilterCalendar, toggleShowFilterCalendar] = useState(false);
	const [eventAppFilterActive, toggleEventAppFilterActive] = useState(false);
	const [eventDateFilterActive, toggleEventDateFilterActive] = useState(false);
	const [showEventFilterCalendar, toggleShowEventFilterCalendar] = useState(false);
	const [eventApp, setEventApp] = useState('');
	const [OrganisationDetails, setOrganisationDetails] = useState({
		database: {
			dsn: ''
		},
		queue: {
			type: '',
			redis: {
				dsn: ''
			}
		},
		server: {
			http: {
				port: 0
			}
		},
		strategy: {
			type: '',
			default: {
				intervalSeconds: 0,
				retryLimit: 0
			}
		},
		signature: {
			header: '',
			hash: ''
		}
	});
	const [eventDeliveryAtempt, setEventDeliveryAtempt] = useState({
		ip_address: '',
		http_status: '',
		api_version: '',
		updated_at: 0,
		deleted_at: 0,
		response_data: '',
		response_http_header: {},
		request_http_header: {}
	});
	const [detailsItem, setDetailsItem] = useState();
	const [filterFrequency, setFilterFrequency] = useState('');
	const [filterDates, setFilterDates] = useState([
		{
			startDate: new Date(new Date().setDate(new Date().getDate() - 30)),
			endDate: new Date(),
			key: 'selection'
		}
	]);
	const [eventFilterDates, setEventFilterDates] = useState([
		{
			startDate: new Date(),
			endDate: new Date(),
			key: 'selection'
		}
	]);

	const [options] = useState({
		plugins: {
			legend: {
				display: false
			}
		},
		scales: {
			xAxis: {
				display: true,
				grid: {
					display: false
				}
			}
		}
	});

	const setEventsDisplayed = events => {
		const dateCreateds = events.map(event => getDate(event.created_at));
		const uniqueDateCreateds = [...new Set(dateCreateds)];
		const displayedEvents = [];
		uniqueDateCreateds.forEach(eventDate => {
			const filteredEventDate = events.filter(event => getDate(event.created_at) === eventDate);
			const eventsItem = { date: eventDate, events: filteredEventDate };
			displayedEvents.push(eventsItem);
		});
		setDisplayedEvents(displayedEvents);
	};

	const setDateForFilter = ({ startDate, endDate }) => {
		if (!endDate && !startDate) return { startDate: '', endDate: '' };
		startDate = String(moment(`${moment(startDate).format('YYYY[-]MM[-]DD')} 00:00:00`).toISOString(true)).split('.')[0];
		endDate = String(moment(`${moment(endDate).format('YYYY[-]MM[-]DD')} 23:59:59`).toISOString(true)).split('.')[0];
		return { startDate, endDate };
	};

	const getEvents = useCallback(
		async ({ page, eventsData, dates }) => {
			toggleShowEventFilterCalendar(false);

			if (!dates) dates = [{ startDate: null, endDate: null }];

			const dateFromPicker = dates[0];
			const { startDate, endDate } = setDateForFilter(dateFromPicker);

			try {
				const eventsResponse = await (
					await request({
						url: `/events?sort=AESC&page=${page || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${eventApp}`,
						method: 'GET'
					})
				).data;

				if (eventsData && eventsData?.pagination?.next === page) {
					const content = [...eventsData.content, ...eventsResponse.data.content];
					const pagination = eventsResponse.data.pagination;
					setEventsData({ content, pagination });
					setEventsDisplayed(content);
					return;
				}

				setEventsData(eventsResponse.data);
				setEventsDisplayed(eventsResponse.data.content);
			} catch (error) {
				return error;
			}
		},
		[eventApp]
	);

	const getApps = useCallback(async ({ page, appsData }) => {
		try {
			const appsResponse = await (
				await request({
					url: `/apps?sort=AESC&page=${page || 1}&perPage=10`
				})
			).data;

			if (appsData?.pagination?.next === page) {
				const content = [...appsData.content, ...appsResponse.data.content];
				const pagination = appsResponse.data.pagination;
				setAppsData({ content, pagination });
				return;
			}
			setAppsData(appsResponse.data);
		} catch (error) {
			return error;
		}
	}, []);

	const getDelieveryAttempts = async eventId => {
		try {
			const deliveryAttemptsResponse = await (
				await request({
					url: `/events/${eventId}/deliveryattempts`
				})
			).data;
			setEventDeliveryAtempt(deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1]);
			Prism.highlightAll();
		} catch (error) {
			return error;
		}
	};

	const retryEvent = async ({ eventId, appId, e, index }) => {
		e.stopPropagation();
		const retryButton = document.querySelector(`#event${index} button`);
		retryButton.classList.add(['spin', 'disable_action']);
		retryButton.disabled = true;

		try {
			await (
				await request({
					method: 'PUT',
					url: `/apps/${appId}/events/${eventId}/resend`
				})
			).data;
			showNotification({ message: 'Retry Request Sent' });
			retryButton.classList.remove(['spin', 'disable_action']);
			retryButton.disabled = false;
			getEvents({ page: events.pagination.page });
		} catch (error) {
			showNotification({ message: error.response.data.message });
			retryButton.classList.remove(['spin', 'disable_action']);
			retryButton.disabled = false;
			return error;
		}
	};

	const toggleActiveTab = tab => {
		setActiveTab(tab);
		setDetailsItem();
		setEventDeliveryAtempt({
			ip_address: '',
			http_status: '',
			api_version: '',
			updated_at: 0,
			deleted_at: 0
		});
	};

	useEffect(() => {
		const getOrganisationDetails = async () => {
			try {
				const organisationDetailsResponse = await (
					await request({
						url: `/dashboard/config`
					})
				).data;
				setOrganisationDetails(organisationDetailsResponse.data);
			} catch (error) {
				return error;
			}
		};

		const fetchDashboardData = async () => {
			try {
				const { startDate, endDate } = setDateForFilter(filterDates[0]);
				const dashboardResponse = await request({
					url: `/dashboard/summary?startDate=${startDate}&endDate=${endDate}&type=${filterFrequency || 'daily'}`
				});
				setDashboardData(dashboardResponse.data.data);

				const chartData = dashboardResponse.data.data.message_data;
				const labels = [0, ...chartData.map(label => label.data.date)];
				const dataSet = [0, ...chartData.map(label => label.count)];
				const data = {
					labels,
					datasets: [
						{
							data: dataSet,
							fill: false,
							borderColor: '#477DB3',
							tension: 0.5,
							yAxisID: 'yAxis',
							xAxisID: 'xAxis'
						}
					]
				};

				if (!Chart.getChart('chart') || !Chart.getChart('chart')?.canvas) {
					new Chart(document.getElementById('chart'), { type: 'line', data, options });
				} else {
					const currentChart = Chart.getChart('chart');
					currentChart.data.labels = labels;
					currentChart.data.datasets[0].data = dataSet;
					currentChart.update();
				}
			} catch (error) {
				return error;
			}
		};

		fetchDashboardData();
		getOrganisationDetails();
		getApps({ page: 1 });
		getEvents({ page: 1 });
	}, [options, activeTab, filterDates, filterFrequency, getEvents, getApps]);

	return (
		<div className="dashboard">
			<header className="dashboard--header">
				<div className="dashboard--header--container">
					<div className="logo">
						<img src={ConvoyLogo} alt="convoy logo" />
					</div>

					<button className="user" onClick={() => toggleShowDropdown(!showDropdown)}>
						<div>
							<div className="icon">O</div>
							<div className="name">{JSON.parse(authDetails).username}</div>
						</div>
						<img src={AngleArrowDownIcon} alt="arrow down icon" />
						{showDropdown && (
							<div className="dropdown organisations">
								<ul>
									<li onClick={() => logout()}>Logout</li>
								</ul>
							</div>
						)}
					</button>
				</div>
			</header>

			<div className="dashboard--page">
				<div className={`filter ${showFilterCalendar ? 'show-calendar' : ''}`}>
					<div>Filter by:</div>
					<button className="filter--button" onClick={() => toggleShowFilterCalendar(!showFilterCalendar)}>
						<img src={CalendarIcon} alt="calender icon" />
						<div>
							{getDate(filterDates[0].startDate)} - {getDate(filterDates[0].endDate)}
						</div>
						<img src={AngleArrowDownIcon} alt="arrow down icon" />
					</button>
					<DateRange onChange={item => setFilterDates([item.selection])} moveRangeOnFirstSelection={false} ranges={filterDates} />

					<div className="select">
						<select value={filterFrequency} onChange={event => setFilterFrequency(event.target.value)} aria-label="frequency">
							<option value="daily">Daily</option>
							<option value="weekly">Weekly</option>
							<option value="monthly">Monthly</option>
							<option value="yearly">Yearly</option>
						</select>
					</div>
				</div>

				<div className="dashboard--page--details">
					<div className="card dashboard--page--details--chart">
						<ul>
							<li className="messages">
								<img src={MessageIcon} alt="message icon" />
								<div className="metric">
									<div>{dashboardData.messages_sent}</div>
									<div>{dashboardData.messages_sent === 1 ? 'Event' : 'Events'} Sent</div>
								</div>
							</li>
							<li className="apps">
								<img src={AppsIcon} alt="apps icon" />
								<div className="metric">
									<div>{dashboardData.apps}</div>
									<div>{dashboardData.apps === 1 ? 'App' : 'Apps'}</div>
								</div>
							</li>
						</ul>

						<div>
							<h3>Events Sent</h3>
							<canvas id="chart" width="400" height="200"></canvas>
						</div>
					</div>

					<div className="card has-title dashboard--page--details--credentials">
						<div className="card--title">
							<h2>Organization Details</h2>
						</div>

						<ul className="card--container">
							<li className="list-item">
								<div className="list-item--label">
									DB URL
									<div className="list-item--item">{OrganisationDetails.database.dsn}</div>
								</div>
								<button onClick={() => copyText(OrganisationDetails.database.dsn)}>
									<img src={CopyIcon} alt="copy icon" />
								</button>
							</li>

							<li className="list-item">
								<div className="list-item--label">
									Queue
									<div className="list-item--item">{OrganisationDetails.queue.redis.dsn}</div>
								</div>
								<button onClick={() => copyText(OrganisationDetails.queue.redis.dsn)}>
									<img src={CopyIcon} alt="copy icon" />
								</button>
							</li>

							<li className="list-item">
								<div className="list-item--label">
									Server
									<div className="list-item--item">http://localhost:{OrganisationDetails.server.http.port}</div>
								</div>
								<button onClick={() => copyText('http://localhost:' + OrganisationDetails.server.http.port)}>
									<img src={CopyIcon} alt="copy icon" />
								</button>
							</li>

							<li className="list-item">
								<div className="list-item--label">
									Request interval Seconds
									<div className="list-item--item">{OrganisationDetails.strategy.default.intervalSeconds}s</div>
								</div>
							</li>

							<li className="list-item">
								<div className="list-item--label">
									Retry limit
									<div className="list-item--item">{OrganisationDetails.strategy.default.retryLimit}</div>
								</div>
							</li>

							<li className="list-item">
								<div className="list-item--label">
									Signature header
									<div className="list-item--item">{OrganisationDetails.signature.header}</div>
								</div>
							</li>

							<li className="list-item">
								<div className="list-item--label">
									Signature hash
									<div className="list-item--item">{OrganisationDetails.signature.hash}</div>
								</div>
							</li>
						</ul>
					</div>
				</div>

				<section className="card dashboard--logs">
					<div className="dashboard--logs--tabs">
						<div className="dashboard--logs--tabs--head tabs">
							<div className="tabs">
								{tabs.map((tab, index) => (
									<button onClick={() => toggleActiveTab(tab)} key={index} className={'clear tab ' + (activeTab === tab ? 'active' : '')}>
										{tab}
									</button>
								))}
							</div>

							{activeTab === 'events' && (
								<div className="filter">
									<button className={'filter--button ' + (eventDateFilterActive ? 'active' : '')} onClick={() => toggleShowEventFilterCalendar(!showEventFilterCalendar)}>
										<img src={CalendarIcon} alt="calender icon" />
										<div>Date</div>
										<img src={AngleArrowDownIcon} alt="arrow down icon" />
									</button>
									{showEventFilterCalendar && (
										<div className="date-filter--container">
											<DateRange onChange={item => setEventFilterDates([item.selection])} editableDateInputs={true} moveRangeOnFirstSelection={false} ranges={eventFilterDates} />
											<div className="button-container">
												<button
													className="primary"
													onClick={() => {
														getEvents({ dates: eventFilterDates });
														toggleEventDateFilterActive(true);
													}}>
													Apply
												</button>
												<button
													className="primary outline"
													onClick={() => {
														getEvents({ page: 1 });
														toggleEventDateFilterActive(false);
													}}>
													Clear
												</button>
											</div>
										</div>
									)}

									<div className="select">
										<select
											value={eventApp}
											className={eventAppFilterActive ? 'active' : ''}
											onChange={event => {
												setEventApp(event.target.value);
												toggleEventAppFilterActive(!!event.target.value);
											}}
											aria-label="frequency">
											<option value="">All Apps</option>
											{apps.content.map((app, index) => (
												<option key={index} value={app.uid}>
													{app.name}
												</option>
											))}
										</select>
									</div>
								</div>
							)}
						</div>

						<div className="table">
							{displayedEvents.length > 0 && activeTab && activeTab === 'events' && (
								<div>
									<table>
										<thead>
											<tr className="table--head">
												<th scope="col">Status</th>
												<th scope="col">Event Type</th>
												<th scope="col">Attempts</th>
												<th scope="col">Next Retry</th>
												<th scope="col">Created At</th>
												<th scope="col">Next Entry</th>
											</tr>
										</thead>
										<tbody>
											{displayedEvents.map((eventGroup, index) => (
												<React.Fragment key={'eventGroup' + index}>
													<tr className="table--date-row">
														<td>
															<div>{eventGroup.date}</div>
														</td>
														<td></td>
														<td></td>
														<td></td>
														<td></td>
														<td></td>
													</tr>
													{eventGroup.events.map((event, index) => (
														<tr
															key={index}
															onClick={() => {
																Prism.highlightAll();
																setDetailsItem(event);
																getDelieveryAttempts(event.uid);
															}}
															className={event.uid === detailsItem?.uid ? 'active' : ''}
															id={'event' + index}>
															<td>
																<div className="has-retry">
																	{event.metadata.num_trials > event.metadata.retry_limit && <img src={RetryIcon} alt="retry icon" title="manual retried" />}
																	<div className={'tag tag--' + event.status}>{event.status}</div>
																</div>
															</td>
															<td>
																<div>{event.event_type}</div>
															</td>
															<td>
																<div>{event.metadata.num_trials}</div>
															</td>
															<td>
																<div>{event.metadata.num_trials < event.metadata.retry_limit && event.status !== 'Success' ? getTime(event.metadata.next_send_time) : '-'}</div>
															</td>
															<td>
																<div>{getTime(event.created_at)}</div>
															</td>
															<td>
																<div>
																	<button
																		disabled={event.status === 'Success' || event.status === 'Scheduled'}
																		className={'primary has-icon icon-left ' + (event.status === 'Success' || event.status === 'Scheduled' ? 'disable_action' : '')}
																		onClick={e => retryEvent({ eventId: event.uid, appId: event.app_id, e, index })}>
																		<img src={RefreshIcon} alt="refresh icon" />
																		Retry
																	</button>
																</div>
															</td>
														</tr>
													))}
												</React.Fragment>
											))}
										</tbody>
									</table>

									{events.pagination.totalPage > 1 && (
										<div className=" table--load-more button-container margin-top center">
											<button
												className={'primary clear has-icon icon-left ' + (events.pagination.page === events.pagination.totalPage ? 'disable_action' : '')}
												disabled={events.pagination.page === events.pagination.totalPage}
												onClick={() => getEvents({ page: events.pagination.page + 1, eventsData: events, dates: eventDateFilterActive ? eventFilterDates : null })}>
												<img src={ArrowDownIcon} alt="arrow down icon" />
												Load more
											</button>
										</div>
									)}
								</div>
							)}

							{activeTab === 'events' && displayedEvents.length === 0 && (
								<div className="empty-state">
									<img src={EmptyStateImage} alt="empty state" />
									<p>No {activeTab} to show here</p>
								</div>
							)}

							{apps.content.length > 0 && activeTab && activeTab === 'apps' && (
								<div>
									<table>
										<thead>
											<tr className="table--head">
												<th scope="col">Name</th>
												<th scope="col">Created</th>
												<th scope="col">Updated</th>
												<th scope="col">Events No</th>
												<th scope="col">Endpoints No</th>
												<th scope="col"></th>
											</tr>
										</thead>
										<tbody>
											{apps.content.map((app, index) => (
												<tr key={index} onClick={() => setDetailsItem(app)} className={app.uid === detailsItem?.uid ? 'active' : ''}>
													<td>
														<div>{app.name}</div>
													</td>
													<td>
														<div>{getDate(app.created_at)}</div>
													</td>
													<td>
														<div>{getDate(app.updated_at)}</div>
													</td>
													<td>
														<div>{app.events}</div>
													</td>
													<td>
														<div>{app.endpoints.length}</div>
													</td>
													<td>
														<div>
															<button
																disabled={app.events <= 0}
																title="view events"
																className={'primary has-icon icon-left ' + (app.events <= 0 ? 'disable_action' : '')}
																onClick={e => {
																	e.stopPropagation();
																	setEventApp(app.uid);
																	toggleActiveTab('events');
																	toggleEventAppFilterActive(true);
																}}>
																<img src={ViewEventsIcon} alt="view events icon" />
																Events
															</button>
														</div>
													</td>
												</tr>
											))}
										</tbody>
									</table>

									{apps.pagination.totalPage > 1 && (
										<div className="table--load-more button-container margin-top center">
											<button
												className={'primary clear has-icon icon-left ' + (apps.pagination.page === apps.pagination.totalPage ? 'disable_action' : '')}
												disabled={apps.pagination.page === apps.pagination.totalPage}
												onClick={() => getApps({ page: apps.pagination.page + 1, appsData: apps })}>
												<img src={ArrowDownIcon} alt="arrow down icon" />
												Load more
											</button>
										</div>
									)}
								</div>
							)}

							{apps.content.length === 0 && activeTab === 'apps' && (
								<div className="empty-state">
									<img src={EmptyStateImage} alt="empty state" />
									<p>No eve to show here</p>
								</div>
							)}
						</div>
					</div>

					{detailsItem && (
						<div className="dashboard--logs--details">
							<h3>Details</h3>
							<ul className="dashboard--logs--details--meta">
								{eventDeliveryAtempt && eventDeliveryAtempt.ip_address && (
									<React.Fragment>
										<li>
											<div className="label">IP Address</div>
											<div className="value color">{eventDeliveryAtempt.ip_address}</div>
										</li>
										<li>
											<div className="label">HTTP Status</div>
											<div className="value">{eventDeliveryAtempt.http_status}</div>
										</li>
										<li>
											<div className="label">API Version</div>
											<div className="value color">{eventDeliveryAtempt.api_version}</div>
										</li>
									</React.Fragment>
								)}
								<li>
									<div className="label">Date Created</div>
									<div className="value">{getDate(detailsItem.created_at)}</div>
								</li>
								<li>
									<div className="label">Last Updated</div>
									<div className="value">{getDate(detailsItem.updated_at)}</div>
								</li>
								{detailsItem.support_email && (
									<li>
										<div className="label">Support Email</div>
										<div className="value">{detailsItem.support_email}</div>
									</li>
								)}
							</ul>

							{activeTab === 'events' && (
								<ul className="tabs">
									{eventDetailsTabs.map(tab => (
										<li className={'tab ' + (eventDetailsActiveTab === tab.id ? 'active' : '')} key={tab.id}>
											<button className="primary outline" onClick={() => setEventDetailsActiveTab(tab.id)}>
												{tab.label}
											</button>
										</li>
									))}
								</ul>
							)}

							{activeTab === 'events' && (
								<div className="dashboard--logs--details--req-res">
									<div className={'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'data' ? 'show' : '')}>
										<pre className="line-numbers">
											<code className="lang-javascript">{detailsItem.data ? JSON.stringify(detailsItem.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:') : ''}</code>
										</pre>
									</div>

									<div className={'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'response' ? 'show' : '')}>
										<h3>Header</h3>
										<pre className="line-numbers">
											<code className="lang-javascript">
												{eventDeliveryAtempt?.response_http_header
													? JSON.stringify(eventDeliveryAtempt.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:')
													: 'No response header was sent'}
											</code>
										</pre>

										<h3>Body</h3>
										<pre className="line-numbers">
											<code className="lang-html">{eventDeliveryAtempt?.response_data ? eventDeliveryAtempt.response_data : ''}</code>
										</pre>
									</div>

									<div className={'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'request' ? 'show' : '')}>
										<h3>Header</h3>
										<pre className="line-numbers">
											<code className="lang-javascript">
												{eventDeliveryAtempt?.request_http_header
													? JSON.stringify(eventDeliveryAtempt.request_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:')
													: 'No request header was sent'}
											</code>
										</pre>
									</div>
								</div>
							)}

							{activeTab === 'apps' && (
								<React.Fragment>
									<h4>App Event Endpoints</h4>
									<ul className="dashboard--logs--details--endpoints">
										{detailsItem.endpoints &&
											detailsItem.endpoints.map((endpoint, index) => (
												<li key={index}>
													<h5>{endpoint.description}</h5>
													<p>
														<img src={LinkIcon} alt="link icon" />
														{endpoint.target_url}
													</p>
												</li>
											))}
									</ul>
								</React.Fragment>
							)}
						</div>
					)}
				</section>
			</div>
		</div>
	);
}

export { DashboardPage };