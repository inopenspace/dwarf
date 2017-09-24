/* jshint node: true */

module.exports = function(environment) {
  var ENV = {
    modulePrefix: 'dwarf',
    environment: environment,
    rootURL: '/',
    locationType: 'hash',
    EmberENV: {
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. 'with-controller': true
      }
    },

    APP: {
      ApplicationName: "Expanse",
      // API host and port
      ApiUrl: '//big-dwarf.com/',

      // HTTP mining endpoint
      HttpHost: 'http://big-dwarf.com',
      HttpPort: 6666,

      // Stratum mining endpoint
      StratumHost: 'big-dwarf.com',
      StratumPort: 6006,

      // Fee and payout details
      PoolFee: '1%',
      PayoutThreshold: '0.5 Ether',

      // For network hashrate (change for your favourite fork)
      BlockTime: 14.4
    }
  };

  if (environment === 'development') {
    /* Override ApiUrl just for development, while you are customizing
      frontend markup and css theme on your workstation.
    */
    ENV.APP.ApiUrl = 'http://big-dwarf.com:6060/';
    // ENV.APP.LOG_RESOLVER = true;
    // ENV.APP.LOG_ACTIVE_GENERATION = true;
    // ENV.APP.LOG_TRANSITIONS = true;
    // ENV.APP.LOG_TRANSITIONS_INTERNAL = true;
    // ENV.APP.LOG_VIEW_LOOKUPS = true;
  }

  if (environment === 'test') {
    // Testem prefers this...
    ENV.locationType = 'none';

    // keep test console output quieter
    ENV.APP.LOG_ACTIVE_GENERATION = false;
    ENV.APP.LOG_VIEW_LOOKUPS = true;

    ENV.APP.rootElement = '#ember-testing';
  }

  if (environment === 'production') {
    console.log("PROD env.");
  }

  return ENV;
};
