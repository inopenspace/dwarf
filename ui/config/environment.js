/* jshint node: true */

module.exports = function(environment) {
  var ENV = {
    modulePrefix: 'dwarf',
    environment: environment,
    rootURL: '/',
    locationType: 'hash',
    applicationName:"Big Dwarf",
    EmberENV: {
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. 'with-controller': true
      }
    },
    app:'music',
    APP: {}
  };
  ENV.APP=require('./'+ENV.app);
  if (environment === 'development') {

    ENV.APP.LOG_RESOLVER = true;
     ENV.APP.LOG_ACTIVE_GENERATION = true;
   ENV.APP.LOG_TRANSITIONS = true;
     ENV.APP.LOG_TRANSITIONS_INTERNAL = true;
    ENV.APP.LOG_VIEW_LOOKUPS = true;
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
