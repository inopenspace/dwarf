import Ember from 'ember';
import config from '../config/environment';

export default Ember.Route.extend({

  actions: {
    lookup(login) {
      if (!Ember.isEmpty(login)) {
        return this.transitionTo('account', login);
      }
    }
  },
  model: function () {
    var url = 'https://min-api.cryptocompare.com/data/price?fsym=' + config.coinName + '&tsyms=BTC,USD';
    return Ember.$.getJSON(url).then(function (data) {
      data.blockExplorerUrl = config.blockExplorerUrl;

      return data;
    });
  },
  setupController: function (controller, model) {
    this._super(controller, model);

    Ember.run.later(this, this.refresh, 5000);
  }
});
