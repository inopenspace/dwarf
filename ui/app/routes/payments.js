import Ember from 'ember';
import Payment from "../models/payment";
import config from '../config/environment';
export default Ember.Route.extend({

	model: function() {
    var url = config.APP.ApiUrl + 'api/payments';
    let promise=Ember.$.getJSON(url).then(function(data) {
			if (data.payments) {
				data.payments = data.payments.map(function(p) {
					return Payment.create(p);
				});
        data.config = config;
			}
			return data;
    });
    return promise.catch(function (error){
      console.log("Request error:"+error.statusText);
    });
	},

  setupController: function(controller, model) {
    this._super(controller, model);
    Ember.run.later(this, this.refresh, 5000);
  }
});
