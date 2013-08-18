// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

(function($) {
	'use strict';

	$(function() {
		$(".cps-go-back").click(function() {
			history.go(-1);
		});
	});

})(window.jQuery);
