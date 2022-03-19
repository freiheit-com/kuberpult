const rewire = require('rewire');
const defaults = rewire('react-scripts/scripts/build.js');
const config = defaults.__get__('config');
if( process.env.CACHE_DIR ){
	config.cache.cacheDirectory = process.env.CACHE_DIR;
}
