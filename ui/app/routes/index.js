import Ember from 'ember';

export default Ember.Route.extend({
  actions: {
    lookup(login) {
      if (!Ember.isEmpty(login)) {
        return this.transitionTo('account', login);
      }
    }
  },
  model: function() {
    var url = 'https://min-api.cryptocompare.com/data/price?fsym=MUSIC&tsyms=BTC,USD';
    return Ember.$.getJSON(url).then(function(data) {

      return data;
    });
  },
  setupController: function(controller, model) {
    this._super(controller, model);

    Ember.run.later(this, this.refresh, 5000);
  }
});
