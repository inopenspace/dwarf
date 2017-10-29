import Ember from 'ember';
import config from '../config/environment';

export default Ember.Route.extend({
  intl: Ember.inject.service(),

  beforeModel() {
    this.get('intl').setLocale('en-us');
  },
  actions: {
    error: function(error) {
      this.send('ajaxError', error);
    },

    ajaxError: function(error) {
      this.ajaxError(error);
      Ember.run.later(this, this.refresh, 5000);
    },
  },
	model: function() {
    var url = config.APP.ApiUrl + 'api/stats';
    return Ember.$.getJSON(url).then(function(data) {
      data.coinName=config.coinName;
      data.applicationName=config.applicationName;
      return Ember.Object.create(data);
    });
	},

  setupController: function(controller, model) {
    this._super(controller, model);
    Ember.run.later(this, this.refresh, 5000);
  }
});
