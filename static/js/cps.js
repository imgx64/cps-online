// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

(function($) {
	"use strict";

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
				$(this).closest("td").prevAll().find(".cps-grid").not("[disabled]").last().select()
				break;

			case 38: // up
				$(this).closest("tr").prev().children().eq(col).find(".cps-grid").select()
				break;

			case 39: // right
				$(this).closest("td").nextAll().find(".cps-grid").not("[disabled]").first().select()
				break;

			case 13: // Enter
			case 40: // down
				$(this).closest("tr").next().children().eq(col).find(".cps-grid").select()
				break;

			default: return; // exit this handler for other keys
			}
			e.preventDefault(); // prevent the default action (scroll / move caret)
		});

		// Prevent accidental navigation away
		function setConfirmUnload(on) {
			window.onbeforeunload = on ? unloadMessage : null;
		}

		function unloadMessage() {
			return("You have entered new data on this page. " +
					"If you navigate away from this page without " +
					"first saving your data, the changes will be lost.");
		}
		$(".cps-grid").bind("change", function() { setConfirmUnload(true); });
		$(":input[type=submit]").click(function() { setConfirmUnload(false); });


	});

})(window.jQuery);
