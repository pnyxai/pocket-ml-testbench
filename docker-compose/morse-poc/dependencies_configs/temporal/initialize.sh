#!/bin/bash

temporal operator namespace create pocket-ml-testbench

sleep 3


all_services="A100,A101,A102,A103"
two_services="A100,A103"
one_services="A100"
# change this if you want a different set of datasets, by default it create everything
keys_services=$one_services

keys_services_array=(${keys_services//,/ })

everything="arc_challenge,hellaswag,truthfulqa_mc2,mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions,winogrande,gsm8k"
mmlu="mmlu_abstract_algebra,mmlu_anatomy,mmlu_astronomy,mmlu_business_ethics,mmlu_clinical_knowledge,mmlu_college_biology,mmlu_college_chemistry,mmlu_college_computer_science,mmlu_college_mathematics,mmlu_college_medicine,mmlu_college_physics,mmlu_computer_security,mmlu_conceptual_physics,mmlu_econometrics,mmlu_electrical_engineering,mmlu_elementary_mathematics,mmlu_formal_logic,mmlu_global_facts,mmlu_high_school_biology,mmlu_high_school_chemistry,mmlu_high_school_computer_science,mmlu_high_school_european_history,mmlu_high_school_geography,mmlu_high_school_government_and_politics,mmlu_high_school_macroeconomics,mmlu_high_school_mathematics,mmlu_high_school_microeconomics,mmlu_high_school_physics,mmlu_high_school_psychology,mmlu_high_school_statistics,mmlu_high_school_us_history,mmlu_high_school_world_history,mmlu_human_aging,mmlu_human_sexuality,mmlu_international_law,mmlu_jurisprudence,mmlu_logical_fallacies,mmlu_machine_learning,mmlu_management,mmlu_marketing,mmlu_medical_genetics,mmlu_miscellaneous,mmlu_moral_disputes,mmlu_moral_scenarios,mmlu_nutrition,mmlu_philosophy,mmlu_prehistory,mmlu_professional_accounting,mmlu_professional_law,mmlu_professional_medicine,mmlu_professional_psychology,mmlu_public_relations,mmlu_security_studies,mmlu_sociology,mmlu_us_foreign_policy,mmlu_virology,mmlu_world_religions"
heavy="arc_challenge,hellaswag,truthfulqa_mc2,winogrande,gsm8k"
one="mmlu_college_biology"
# change this if you want a different set of datasets, by default it create everything
keys=$one

json_array=$(printf ',"%s"' "${key_array[@]}")
json_array="[${json_array:1}]"

# Convert string to array
IFS=',' read -ra key_array <<< "$keys"

for key in "${key_array[@]}"; do
  # set a workflow id to prevent it been created twice
  temporal workflow start \
    --workflow-id "register-$key" \
    --id-reuse-policy "AllowDuplicateFailedOnly" \
    --task-queue 'sampler' \
    --type 'Register' \
    --input "{\"framework\": \"lmeh\", \"tasks\": \"$key\"}" \
    --execution-timeout 7200 \
    --task-timeout 3600 \
    --namespace 'pocket-ml-testbench'
done

# this time will be more or less depending on internet speed, amount of replicas of sampler and resources assigned to it
# this is an estimate after test with 30MB/s, 3 replicas with 2 Cores each

for service in "${keys_services_array[@]}"; do
  for key in "${key_array[@]}"; do
    temporal schedule create \
        --schedule-id "lmeh-$key-$service" \
        --workflow-id "lmeh-$key-$service" \
        --namespace 'pocket-ml-testbench' \
        --workflow-type 'Manager' \
        --task-queue 'manager' \
        --interval '2m' \
        --overlap-policy "Skip" \
        --execution-timeout 120 \
        --task-timeout 120 \
        --input "{\"service\":\"$service\", \"tests\": [{\"framework\": \"lmeh\", \"tasks\": [\"$key\"]}]}"
  done
done

for service in "${keys_services_array[@]}"; do
  temporal schedule create \
        --schedule-id "lmeh-tokenizer-$service" \
        --workflow-id "lmeh-tokenizer-$service" \
        --namespace 'pocket-ml-testbench' \
        --workflow-type 'Manager' \
        --task-queue 'manager' \
        --interval '2m' \
        --overlap-policy "Skip" \
        --execution-timeout 120 \
        --task-timeout 120 \
        --input "{\"service\":\"$service\", \"tests\": [{\"framework\": \"signatures\", \"tasks\": [\"tokenizer\"]}]}"
done

for service in "${keys_services_array[@]}"; do
  temporal schedule create \
        --schedule-id "lmeh-config-$service" \
        --workflow-id "lmeh-config-$service" \
        --namespace 'pocket-ml-testbench' \
        --workflow-type 'Manager' \
        --task-queue 'manager' \
        --interval '2m' \
        --overlap-policy "Skip" \
        --execution-timeout 120 \
        --task-timeout 120 \
        --input "{\"service\":\"$service\", \"tests\": [{\"framework\": \"signatures\", \"tasks\": [\"config\"]}]}"
done

for service in "${keys_services_array[@]}"; do
  temporal schedule create \
      --schedule-id "f3abbe313689a603a1a6d6a43330d0440a552288-$service" \
      --workflow-id "f3abbe313689a603a1a6d6a43330d0440a552288-$service" \
      --namespace 'pocket-ml-testbench' \
      --workflow-type 'Requester' \
      --task-queue 'requester' \
      --interval '1m' \
      --overlap-policy "Skip" \
      --execution-timeout 350 \
      --task-timeout 175 \
      --input "{\"app\":\"f3abbe313689a603a1a6d6a43330d0440a552288\",\"service\":\"$service\"}"
done

temporal schedule create \
    --schedule-id 'lookup-done-tasks' \
    --workflow-id 'lookup-done-tasks' \
    --namespace 'pocket-ml-testbench' \
    --workflow-type 'LookupTasks' \
    --task-queue 'evaluator' \
    --interval '1m' \
    --overlap-policy "Skip" \
    --execution-timeout 350 \
    --task-timeout 175