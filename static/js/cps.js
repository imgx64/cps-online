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

		$(".cps-grid").keydown(function(e) {
			var col = $(this).closest("td").index()

			switch(e.which) {
			case 37: // left
				$(this).closest("td").prevAll().find(".cps-grid").not("[disabled]").last().focus()
				break;

			case 38: // up
				$(this).closest("tr").prev().children().eq(col).find(".cps-grid").focus()
				break;

			case 39: // right
				$(this).closest("td").nextAll().find(".cps-grid").not("[disabled]").first().focus()
				break;

			case 40: // down
				$(this).closest("tr").next().children().eq(col).find(".cps-grid").focus()
				break;

			default: return; // exit this handler for other keys
			}
			e.preventDefault(); // prevent the default action (scroll / move caret)
		});
	});

})(window.jQuery);
