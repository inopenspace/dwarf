import Ember from 'ember';
import Resolver from './resolver';
import loadInitializers from 'ember-load-initializers';
import config from './config/environment';
import AjaxErrorMixin from './mixins/ajax-error';

let App;
Ember.Route.reopen(AjaxErrorMixin);
Ember.Controller.reopen(AjaxErrorMixin);

Ember.Component.reopen({
  _actions: {
    // Passing ajaxError per default
    ajaxError: function(error) {
      this.sendAction('ajaxError', error);
    }
  }
});
Ember.MODEL_FACTORY_INJECTIONS = true;

App = Ember.Application.extend({
  modulePrefix: config.modulePrefix,
  podModulePrefix: config.podModulePrefix,
  Resolver
});

loadInitializers(App, config.modulePrefix);

export default App;
