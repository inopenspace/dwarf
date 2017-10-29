import Ember from 'ember';
import Block from "../models/block";
import config from '../config/environment';
let yestoday = new Date(Date.now() - 86400 * 1000).getTime();
export default Ember.Route.extend({


  model: function() {
    var url = config.APP.ApiUrl + 'api/blocks';
     return Ember.$.getJSON(url).then(function(data) {
      if (data.candidates) {
        data.candidates = data.candidates.map(function(b) {
          return Block.create(b);
        });
      }
      data.todayFoundBlocksCount = 0;
      data.todayCoinsCount=0;
      if (data.immature) {
        data.immature = data.immature.map(function(block) {

          return Block.create(block);
        });
      }

      if (data.matured) {
        data.matured = data.matured.map(function(block) {
          if (block.timestamp * 1000 > yestoday) {
            data.todayFoundBlocksCount++;
            data.todayCoinsCount= data.todayCoinsCount+parseInt(block.reward)* 0.000000000000000001;
          }
          return Block.create(block);
        });
      }
      data.config=config;
      data.BlockExplorerAddress=config.APP.BlockExplorerAddress;
      return data;
    });
  },

  setupController: function(controller, model) {
    this._super(controller, model);
    Ember.run.later(this, this.refresh, 5000);
  }
});
