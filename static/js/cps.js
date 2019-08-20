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

		$(".are-you-sure").click(function(e) {
			if(!window.confirm("Are you sure?")) {
				e.preventDefault();
			}
		});

		$(".cps-js-leave-type").on("change", function(e) {
			$("#EndDate").attr('disabled', true);
			$("#Time").attr('disabled', true);

			if ($(e.target).attr('disabled')) {
				return;
			}

			if (e.target.value == "LoA") {
				$("#EndDate").attr('disabled', false);
				if ($("#EndDate").val() < $("#StartDate").val()) {
					$("#EndDate").val($("#StartDate").val());
				}
				$("#Time").val('00:00');
			} else if (e.target.value == "ED" || e.target.value == "LA") {
				$("#EndDate").val($("#StartDate").val());
				$("#Time").attr('disabled',false);
			}
		});

		$(".cps-js-leave-type").trigger("change");

		$(".add-progress-report-row").click(function(e) {
			var tbody = $("#progress-report-rows-table > tbody");
			var i = tbody.children("tr").length;
			tbody.append(
				$("<tr></tr>").append(
					$("<td></td>").append(
						$("<input></input>", {
							type: "text",
							name: "prr-description-" + i,
							class: "form-control",
							required: "required"
						}),
					),
					$("<td></td>").append(
						$("<select></select>", {
							id: "prr-type-" + i,
							name: "prr-type-" + i,
							class: "form-control",
							required: "required"
						}).append(
							$("<option></option>"),
							$("<option></option>", {value: "Section", text: "Section header"}),
							$("<option></option>", {value: "CMRENA", text: "C, M, R, E, N/A"}),
							$("<option></option>", {value: "Delete", text: "Delete"})
						)
					)
				)
			);
		});

		$(".add-report-classes-row").click(function(e) {
			var template = document.getElementById("add-report-classes-row-template");
			var clone = document.importNode(template.content, true);

			var tbody = document.querySelector("#report-classes-row-table > tbody");
			var rowButton = document.querySelector("#add-report-classes-row-button");
			tbody.insertBefore(clone, rowButton);
		});

		$(".add-report-subjects-row").click(function(e) {
			var template = document.getElementById("add-report-subjects-row-template");
			var clone = document.importNode(template.content, true);

			var tbody = document.querySelector("#report-subjects-row-table > tbody");
			var rowButton = document.querySelector("#add-report-subjects-row-button");
			tbody.insertBefore(clone, rowButton);
		});
	});

})(window.jQuery);
