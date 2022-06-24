import { Injectable } from '@angular/core';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { PrivateService } from 'src/app/private/private.service';
import { HttpService } from 'src/app/services/http/http.service';

@Injectable({
	providedIn: 'root'
})
export class EventDeliveryDetailsService {
	constructor(private http: HttpService, private privateService: PrivateService) {}

	async getEventDeliveryDetails(eventId: string, token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries/${eventId}`,
					method: 'get',
					token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}

	async getEventDeliveryAttempts(eventId: string, token?: string): Promise<HTTP_RESPONSE> {
		return new Promise(async (resolve, reject) => {
			try {
				const response = await this.http.request({
					url: `${token ? '' : this.privateService.urlFactory('org_project')}/eventdeliveries/${eventId}/deliveryattempts`,
					method: 'get',
					token
				});

				return resolve(response);
			} catch (error: any) {
				return reject(error);
			}
		});
	}
}