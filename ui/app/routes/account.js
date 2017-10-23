import Ember from 'ember';
import config from '../config/environment';

export default Ember.Route.extend({
  model: function (params) {
    let url = config.APP.ApiUrl + 'api/accounts/' + params.login;
    let promise= Ember.$.getJSON(url).then(function (data) {
      data.login = params.login;
      data.config = config;
      let url = 'https://min-api.cryptocompare.com/data/price?fsym='+config.APP.coinName+'&tsyms=BTC,USD';
      let statPromise= Ember.$.getJSON(url).then(function (pricingResponse) {
        data.price = {
          btc: pricingResponse.BTC,
          usd: pricingResponse.USD
        };
        data.totalPaidBtc = data.stats.paid * data.price.btc;
        data.totalPaidUsd = data.stats.paid * data.price.usd;
        let total = 0;
        let yestoday = new Date(Date.now() - 86400 * 1000).getTime();
        data.payments.forEach(function (payment) {

          if (payment.timestamp * 1000 > yestoday) {
            total = total + payment.amount;
          }

        });
        data.paidTodayBtc=total* data.price.btc;
        data.paidTodayUsd=total* data.price.usd;

        return Ember.Object.create(data);
      });
      return statPromise.catch(function (error){
        console.log("Request error:"+error.statusText);
      });

    });
    return promise.catch(function (error){
      console.log("Request error:"+error.statusText);
    });
  },

  setupController: function (controller, model) {
    this._super(controller, model);
    Ember.run.later(this, this.refresh, 5000);
  },

  actions: {
    error(error) {
      if (error.status === 404) {
        return this.transitionTo('not-found');
      }
    }
  }
});
