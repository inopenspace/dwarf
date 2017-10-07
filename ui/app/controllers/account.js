import Ember from 'ember';

export default Ember.Controller.extend({
  applicationController: Ember.inject.controller('application'),
  stats: Ember.computed.reads('applicationController.model.stats'),

  roundPercent: Ember.computed('stats', 'model', {
    get() {
      var percent = this.get('model.roundShares') / this.get('stats.roundShares');
      if (!percent) {
        return 0;
      }

      return percent;
    }
  }),
  roundShares: Ember.computed('stats', 'model', {
    get() {
      var percent = this.get('model.roundShares')/1000000000;
      if (!percent) {
        return 0;
      }
      return percent;
    }
  }),
  allShares: Ember.computed('stats', 'model', {
    get() {
      var percent = this.get('stats.roundShares')/1000000000;
      if (!percent) {
        return 0;
      }
      return percent;
    }
  })
});
