// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

(function($) {
	'use strict';

	$(function() {
		$(".cps-go-back").click(function() {
			history.go(-1);
		});

		$(".cps-subject-row").removeClass("active");
		$(".cps-subject-details").hide();

		$(".cps-subject-row").click(function() {
			$(this).toggleClass("active");
			$("#" + $(this).data("subject") + "-details").toggle();
		});
	});

})(window.jQuery);
