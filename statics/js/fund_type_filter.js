(function () {
  function hasSelectedType() {
    return $(".fund-type-option:checked").length > 0;
  }

  function syncAllState() {
    $("#fund-type-all").prop("checked", !hasSelectedType());
  }

  $(document).on("change", "#fund-type-all", function () {
    $(".fund-type-option").prop("checked", false);
  });

  $(document).on("change", ".fund-type-option", function () {
    $("#fund-type-all").prop("checked", !hasSelectedType());
  });

  $(document).on("click", "#fund-type-clear", function () {
    $("#fund-type-all").prop("checked", false);
    $(".fund-type-option").prop("checked", false);
  });

  $(document).on("submit", "#fund-type-filter-form", function () {
    if ($("#fund-type-all").is(":checked")) {
      $(".fund-type-option").prop("checked", false);
    }
  });

  $(document).ready(syncAllState);
})();
