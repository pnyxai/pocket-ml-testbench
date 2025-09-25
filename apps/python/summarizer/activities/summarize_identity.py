from typing import Tuple
from datetime import datetime
from temporalio import activity
from packages.python.common.auto_heartbeater import auto_heartbeater
from app.app import get_app_logger, get_app_config
from packages.python.lmeh.utils.mongodb import MongoOperator
from packages.python.protocol.protocol import PocketNetworkTaxonomySummaryTaskRequest
from packages.python.protocol.protocol import PocketNetworkMongoDBIdentitySummary
from packages.python.protocol.protocol import TaxonomyNodeSummary
import numpy as np
from temporalio.exceptions import ApplicationError
from bson import ObjectId
import pandas as pd
 
# Fraction of equal signatures to count a supplier as proxied by another
EQUALITY_THRESHOLD = 0.75
# Minimum number of signatures to enable the supplier to be evaluated
MIN_SIGNATURES = 5
# Minimum number of samples to retrieve from db
MIN_SAMPLES = 5
# Flag to be used as "last signature" in unique or proxy suppliers
TRACK_FLAG = "UNIQUE_OR_PROXY"
# Flag to be used as "last signature" in suppliers to be ignored
IGNORE_FLAG = "IGNORE_OR_DUPLICATED"

assert MIN_SAMPLES >= MIN_SIGNATURES

@activity.defn
@auto_heartbeater
async def summarize_identity() -> Tuple[bool, str]:
    app_config = get_app_config()
    summary_logger = get_app_logger("summarize_identity")
    config = app_config["config"]

    mongo_client = config["mongo_client"]
    mongo_operator = MongoOperator(client=mongo_client)

    # get all identity signatures
    result = await mongo_operator.get_identity_signatures(MIN_SAMPLES)
    # Convert to pandas
    data_df = pd.DataFrame(list(result))
    if len(data_df) == 0:
        summary_logger.debug(
            f"No signature data to process identity summary."
        )
        return True, "No data"

    # Sort them by supplier id
    data_df = data_df.sort_values("supplier_id").reset_index(drop = True)
    # Get all unique suppliers
    suppliers = data_df['supplier_id'].unique()
    suppliers.sort()

    # Look for duplicates
    proxy_dict = dict()
    skipped_list = list()
    unique_list = list()
    for idx_supplier, this_supplier in enumerate(suppliers):
        if this_supplier in proxy_dict.keys():
            # This supplier is already being proxied by another, no need to look for it
            continue

        # Get this supplier data
        this_supplier_data = data_df.loc[data_df['supplier_id'] == this_supplier]
        # Get the last index with data from this supplier
        last_index = this_supplier_data.index[-1]
        # Get all the supplier signatures
        these_signatures = this_supplier_data['signature'].values

        if len(these_signatures) < MIN_SIGNATURES:
            # Not enough signatures to evaluate, skip for now
            skipped_list.append(this_supplier)
            summary_logger.debug(
                f"Skipping supplier {this_supplier}, not enough signatures"
            )
            continue

        has_proxy = False
        
        # look among the rest of the suppliers
        for other_supplier in suppliers[idx_supplier+1:]:
            # Get this other supplier data
            other_supplier_data = data_df[last_index+1:].loc[data_df['supplier_id'] == other_supplier]
            # Get this other supplier signatures
            other_signatures = other_supplier_data['signature'].values
            # Calculate the fraction of signatures from the first supplier that
            # are equal to the second supplier
            equal_frac = 0
            for this_sign in these_signatures:
                if this_sign in other_signatures:
                    equal_frac +=1
            equal_frac /= len(these_signatures)

            # Check fraction
            if equal_frac>EQUALITY_THRESHOLD:
                # Add this supplier as proxies by another
                proxy_dict[other_supplier] = this_supplier
                # Signal that the first supplier has a proxy
                has_proxy = True

                summary_logger.debug(
                    f"Found proxy pair {this_supplier}:{other_supplier}"
                )
        if not has_proxy:
            # This supplier is unique
            unique_list.append(this_supplier)
            summary_logger.debug(
                    f"Found unique supplier {this_supplier}"
                )

    # Build summary entries and set signature state
    summary_state = dict()
    insert_mongo_summaries = dict()
    for this_supplier in suppliers:

        if this_supplier in skipped_list:
            # No entry, not ready yet
            continue
        if this_supplier in unique_list:
            # This is a unique supplier
            is_unique = True
            is_proxy = False
            proxy_id = this_supplier
            summary_state[this_supplier] = TRACK_FLAG
        elif this_supplier in proxy_dict:
            # this is a proxied supplier
            is_unique = False
            is_proxy = False
            proxy_id = proxy_dict[this_supplier]
            summary_state[this_supplier] = IGNORE_FLAG
        else:
            # This is the proxy of other suppliers
            is_unique = False
            is_proxy = True
            proxy_id = this_supplier
            summary_state[this_supplier] = TRACK_FLAG
        
        # build data to insert in mongo
        insert_mongo_summaries[this_supplier] = PocketNetworkMongoDBIdentitySummary(
                    supplier_id = this_supplier,
                    summary_date = datetime.today().isoformat(),
                    is_unique = is_unique,
                    is_proxy = is_proxy,
                    proxy_id = proxy_id
                ).model_dump(by_alias=True)
            
        
    if len(insert_mongo_summaries) == 0:
        summary_logger.debug(
            f"No identity summary to be created."
        )
        return True, "No summaries yet."
        
    # Upload results to identity summary collection
    try:
        async with mongo_client.start_transaction() as session:
            for this_supplier in insert_mongo_summaries.keys():
                result_dump = insert_mongo_summaries[this_supplier]
                result_dump.pop("_id", None)  # We cannot replace the id
                await mongo_client.db[
                        mongo_operator.identity_summaries
                    ].find_one_and_replace(
                        {
                            "supplier_id": this_supplier,
                        },
                        result_dump,
                        upsert=True,
                        return_document=False,
                        session=session,
                    )
            summary_logger.debug("Identity Summary instances saved to MongoDB successfully.")
    except Exception as e:
        summary_logger.error("Failed to save Identity Summary Instances to MongoDB.")
        summary_logger.error("Exception:", Exception=str(e))
        raise ApplicationError(
            "Failed to save instances to MongoDB.", non_retryable=True
        )
    
    # Update signatures buffer collection with the correct "last signature" state
    try:
        async with mongo_client.start_transaction() as session:
            for this_supplier in summary_state.keys():
                # Get signatures buffer id
                this_supplier_data = data_df.loc[data_df['supplier_id'] == this_supplier]
                this_id = this_supplier_data["_id"].values[0]

                await mongo_client.db["buffers_signatures"].update_one(
                    {"_id": this_id},
                    {"$set": {"last_signature": summary_state[this_supplier]}},
                    session=session,
                )
        summary_logger.debug("Updated signature entries.")
    except Exception as e:
        error_msg = "Failed to update signatures buffers"
        summary_logger.error(
            error_msg,
            error=str(e),
        )
        return False, f"{error_msg}: {str(e)}"


    summary_logger.debug(
        f"Success identity summary."
    )

    return True, ""
