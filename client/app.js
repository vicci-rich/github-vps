(function (angular) {
  'use strict';

  // The client talks to exactly two services: the ticker (markets/assets) and
  // the light node (proof/{sha}). API_BASE is empty when the app is served by
  // the same origin and absolute in tests.
  angular.module('routeApp', [])
    .constant('API_BASE', '')
    .factory('marketsService', ['$http', 'API_BASE', function ($http, base) {
      return {
        markets: function () {
          return $http.get(base + '/markets.json').then(function (resp) { return resp.data; });
        },
        assets: function () {
          return $http.get(base + '/assets.json').then(function (resp) { return resp.data; });
        }
      };
    }])
    .factory('recordService', ['$http', 'API_BASE', function ($http, base) {
      return {
        get: function (sha) {
          return $http.get(base + '/proof/' + sha).then(function (resp) { return resp.data; });
        }
      };
    }])
    .controller('MarketsController', ['marketsService', function (marketsService) {
      var vm = this;
      vm.markets = [];
      vm.error = '';
      vm.refresh = function () {
        marketsService.markets().then(function (rows) {
          vm.markets = rows;
          vm.error = '';
        }, function () {
          vm.error = 'markets unavailable';
        });
      };
      vm.refresh();
    }]);
})(window.angular);
